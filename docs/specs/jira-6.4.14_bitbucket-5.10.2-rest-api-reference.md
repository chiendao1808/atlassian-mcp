# Jira 6.4.14 & Bitbucket Server 5.10.2 — REST API Reference

> Tài liệu thực hành cho hệ thống self-hosted/internal. Nội dung ưu tiên tài liệu Atlassian chính thức, dùng biến môi trường để tránh hard-code credential và nhóm endpoint theo nghiệp vụ.

## 1. Phạm vi và lưu ý tương thích

| Hệ thống | Phiên bản triển khai | REST reference sử dụng | Lưu ý |
| --- | ---: | --- | --- |
| Jira Server | 6.4.14 | Jira REST API **6.4.13**, API version `2` | Kho tài liệu REST chính thức của Atlassian không xuất bản mục `6.4.14`; mục 6.4.x gần nhất là `6.4.13`. Jira có Java API và release notes cho 6.4.14. Vì vậy, mọi endpoint Jira trong file này cần được smoke-test trên instance 6.4.14, nhất là field/parameter phụ thuộc patch hoặc plugin. |
| Bitbucket Server | 5.10.2 | Bitbucket Server REST **5.10.2**, API version `1.0` | Có reference chính thức đúng phiên bản. |

Tài liệu bao phủ các Core REST API thường dùng: auth, metadata, issue/search/project/user của Jira; project/repository/ref/source/commit/pull request/webhook/hook của Bitbucket. Plugin cài thêm có thể cung cấp API riêng dưới namespace khác.

## 2. Mục lục

- [3. Quy ước và biến môi trường](#3-quy-ước-và-biến-môi-trường)
- [4. Xác thực Jira](#4-xác-thực-jira)
- [5. Quy ước chung Jira](#5-quy-ước-chung-jira)
- [6. Jira — System & metadata](#6-jira--system--metadata)
- [7. Jira — Issue CRUD & JQL search](#7-jira--issue-crud--jql-search)
- [8. Jira — Comment, transition, attachment, worklog, watcher](#8-jira--comment-transition-attachment-worklog-watcher)
- [9. Jira — Project, component, version](#9-jira--project-component-version)
- [10. Jira — User & permission](#10-jira--user--permission)
- [11. Xác thực Bitbucket](#11-xác-thực-bitbucket)
- [12. Quy ước chung Bitbucket](#12-quy-ước-chung-bitbucket)
- [13. Bitbucket — Project & repository](#13-bitbucket--project--repository)
- [14. Bitbucket — Branch & tag](#14-bitbucket--branch--tag)
- [15. Bitbucket — Source, commit, compare & diff](#15-bitbucket--source-commit-compare--diff)
- [16. Bitbucket — Pull request & code review](#16-bitbucket--pull-request--code-review)
- [17. Bitbucket — Webhook & repository hook](#17-bitbucket--webhook--repository-hook)
- [18. Error handling, retry và kiểm thử](#18-error-handling-retry-và-kiểm-thử)
- [19. Nguồn chính thức](#19-nguồn-chính-thức)

## 3. Quy ước và biến môi trường

```bash
# Base URL phải chứa context path nếu ứng dụng được deploy dưới /jira hoặc /bitbucket.
export JIRA_BASE='https://jira.internal.example.com/jira'
export JIRA_USER='svc-jira-api'
export JIRA_PASSWORD='***'

export BB_BASE='https://bitbucket.internal.example.com/bitbucket'
export BB_USER='svc-bitbucket-api'
export BB_PASSWORD='***'
export BB_TOKEN='***'      # Personal Access Token của Bitbucket Server 5.10.2
```

| Placeholder | Ý nghĩa | Ví dụ |
| --- | --- | --- |
| `{issueIdOrKey}` | Jira issue numeric ID hoặc key | `10000`, `PROJ-123` |
| `{projectIdOrKey}` | Jira project ID hoặc key | `10000`, `PROJ` |
| `{projectKey}` | Bitbucket project key | `PRJ` |
| `{repositorySlug}` | Slug của repository, thường chữ thường và dấu `-` | `payment-service` |
| `{pullRequestId}` | ID của PR trong target repository | `101` |
| `{path}` | Đường dẫn file; phải URL encode từng segment cần thiết | `src/main/App.java` |
| `{commitId}` | Full/partial SHA hoặc ref nếu endpoint cho phép | `abcdef0123...`, `refs/heads/master` |

### Header chung

| Trường hợp | Header |
| --- | --- |
| Nhận JSON | `Accept: application/json` |
| Gửi JSON | `Content-Type: application/json` |
| Upload multipart Jira | `X-Atlassian-Token: nocheck`; không tự đặt `Content-Type`, để curl tạo boundary |
| Bitbucket PAT | `Authorization: Bearer <token>` hoặc dùng token thay password trong Basic Auth |
| OAuth 1.0a | `Authorization: OAuth ...` với signature được sinh bởi OAuth client |
| Correlation ID nội bộ | Có thể thêm `X-Request-ID`, nhưng chỉ có tác dụng nếu reverse proxy/app đã cấu hình ghi nhận |

> Không dùng `curl -k/--insecure` trong production. Với CA nội bộ, cài CA vào trust store hoặc dùng `--cacert /path/internal-ca.pem`.

## 4. Xác thực Jira

Jira 6.4.x hỗ trợ Basic Auth, cookie-based session, OAuth 1.0a và Trusted Applications. Không áp dụng hướng dẫn API token của Jira Cloud cho Jira Server 6.4.14.

| Cơ chế | Cách gọi | Mẫu curl | Khi nên dùng | Lưu ý |
| --- | --- | --- | --- | --- |
| Basic Auth | HTTP Basic với username/password trên **HTTPS** | `curl -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/myself"` | Script nội bộ đơn giản, service account | Credential được gửi ở mỗi request; dùng tài khoản quyền tối thiểu. Jira có thể từ chối auth nếu CAPTCHA đã kích hoạt. |
| Cookie session | `POST /rest/auth/1/session`, lưu `JSESSIONID`, sau đó gửi cookie | Xem flow bên dưới | Batch nhiều request trong một phiên | Cookie hết hạn theo cấu hình session; bảo vệ file cookie như password. |
| OAuth 1.0a | Consumer key + RSA-SHA1 signature trong `Authorization` header | `curl -H 'Authorization: OAuth oauth_consumer_key="...", oauth_token="...", oauth_signature_method="RSA-SHA1", oauth_timestamp="...", oauth_nonce="...", oauth_version="1.0", oauth_signature="..."' "$JIRA_BASE/rest/api/2/myself"` | Tích hợp dài hạn không muốn lưu password | Cần cấu hình Application Link và thực hiện flow request-token/authorize/access-token. Không tự ghép signature bằng tay. |
| Trusted Applications | Header/signature theo trusted app đã cấu hình | Phụ thuộc cấu hình legacy | Chỉ khi hệ thống đã dùng cơ chế này | Cơ chế legacy; không nên triển khai mới nếu OAuth/PAT qua gateway đáp ứng được. |

### Cookie session flow

```bash
# 1) Đăng nhập và lưu cookie
curl --fail-with-body -sS \
  -c jira-cookies.txt \
  -H 'Accept: application/json' \
  -H 'Content-Type: application/json' \
  --data '{"username":"svc-jira-api","password":"***"}' \
  "$JIRA_BASE/rest/auth/1/session"

# Response đại diện:
# {
#   "session": {"name": "JSESSIONID", "value": "ABC..."},
#   "loginInfo": {
#     "failedLoginCount": 0,
#     "loginCount": 123,
#     "lastFailedLoginTime": "...",
#     "previousLoginTime": "..."
#   }
# }

# 2) Gọi API bằng cookie
curl --fail-with-body -sS \
  -b jira-cookies.txt \
  -H 'Accept: application/json' \
  "$JIRA_BASE/rest/api/2/myself"

# 3) Logout
curl --fail-with-body -sS \
  -b jira-cookies.txt \
  -X DELETE \
  "$JIRA_BASE/rest/auth/1/session"
```

## 5. Quy ước chung Jira

### URI và version

| Thành phần | Giá trị |
| --- | --- |
| Core API | `/rest/api/2/...` |
| Auth/session API | `/rest/auth/1/session` |
| `latest` alias | Có thể tồn tại (`/rest/api/latest/...`) nhưng không nên dùng cho integration cần ổn định; pin `2`. |
| JSON dates | Thường dạng ISO-like với timezone, ví dụ `2016-07-28T09:00:00.000+0700`. |
| Quyền | API thực thi theo quyền của authenticated user; có endpoint tồn tại nhưng trả 404/403 nếu user không nhìn thấy resource. |

### Pagination Jira

| Field | Ý nghĩa |
| --- | --- |
| `startAt` | Offset 0-based. |
| `maxResults` | Kích thước page; server có thể giảm theo giới hạn cấu hình. |
| `total` | Có thể có hoặc không tùy endpoint; không giả định mọi paged response đều có. |
| Chiến lược | Tăng `startAt += số phần tử thực nhận`; dừng khi đủ `total` hoặc page rỗng/nhỏ hơn limit. |

### Error body Jira

```json
{
  "errorMessages": ["Issue does not exist or you do not have permission to see it."],
  "errors": {
    "summary": "You must specify a summary of the issue."
  }
}
```

- `errorMessages`: lỗi cấp request.
- `errors`: map lỗi theo field.
- Luôn log HTTP status, method, URL đã bỏ credential và response body.

## 6. Jira — System & metadata

| Mục đích | Method | Path | Params | Request body | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Thông tin server | GET | /rest/api/2/serverInfo | Query tùy chọn: `doHealthCheck=true\|false`. | Không có body. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/serverInfo" | `{"baseUrl":"https://jira.internal","version":"6.4.14","versionNumbers":[6,4,14],"buildNumber":...,"serverTitle":"Jira Internal"}` |
| Danh sách field | GET | /rest/api/2/field | Không có. | Không có body. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/field" | `[{"id":"summary","name":"Summary","custom":false},{"id":"customfield_10000","name":"Business Unit","custom":true}]` |
| Metadata tạo issue | GET | /rest/api/2/issue/createmeta | `projectIds`, `projectKeys`, `issuetypeIds`, `issuetypeNames`, `expand=projects.issuetypes.fields`. | Không có body. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/createmeta?projectKeys=PROJ&issuetypeNames=Task&expand=projects.issuetypes.fields" | `{"projects":[{"key":"PROJ","issuetypes":[{"id":"3","name":"Task","fields":{"summary":{"required":true}}}]}]}` |
| Metadata sửa issue | GET | /rest/api/2/issue/{issueIdOrKey}/editmeta | Path: `issueIdOrKey`. | Không có body. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/PROJ-123/editmeta" | `{"fields":{"summary":{"required":true},"assignee":{"required":false}}}` |
| Issue types | GET | /rest/api/2/issuetype | Không có. | Không có body. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issuetype" | `[{"self":".../issuetype/3","id":"3","description":"A task","name":"Task","subtask":false}]` |
| Priorities | GET | /rest/api/2/priority | Không có. | Không có body. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/priority" | `[{"id":"3","name":"Major","description":"Major loss of function."}]` |
| Statuses | GET | /rest/api/2/status | Không có. | Không có body. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/status" | `[{"id":"1","name":"Open","statusCategory":{"key":"new","name":"To Do"}}]` |
| Resolutions | GET | /rest/api/2/resolution | Không có. | Không có body. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/resolution" | `[{"id":"1","name":"Fixed","description":"A fix has been checked in."}]` |

## 7. Jira — Issue CRUD & JQL search

| Mục đích | Method | Path | Params | Request body | Headers | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Tạo issue | POST | /rest/api/2/issue | Các field hợp lệ phụ thuộc project, issue type và Create Screen; tra bằng `createmeta`. | `fields` là bắt buộc; project có thể dùng `key`/`id`, issue type dùng `name`/`id`. | `Accept: application/json`; `Content-Type: application/json` | 201 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"fields":{"project":{"key":"PROJ"},"summary":"Lỗi đăng nhập","description":"Mô tả chi tiết lỗi","issuetype":{"name":"Bug"},"priority":{"name":"Major"},"labels":["api","internal"]}}' "$JIRA_BASE/rest/api/2/issue" | `{"id":"10000","key":"PROJ-123","self":"https://jira.internal/rest/api/2/issue/10000"}` |
| Tạo nhiều issue | POST | /rest/api/2/issue/bulk | Không có query chuẩn. | `{"issueUpdates":[{"fields":{...}},{"fields":{...}}]}` | JSON | 201 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"issueUpdates":[{"fields":{"project":{"key":"PROJ"},"summary":"Lỗi đăng nhập","description":"Mô tả chi tiết lỗi","issuetype":{"name":"Bug"},"priority":{"name":"Major"},"labels":["api","internal"]}},{"fields":{"project":{"key":"PROJ"},"summary":"Task 2","issuetype":{"name":"Task"}}}]}' "$JIRA_BASE/rest/api/2/issue/bulk" | `{"issues":[{"id":"10000","key":"PROJ-123","self":"..."},{"id":"10001","key":"PROJ-124","self":"..."}],"errors":[]}` |
| Đọc issue | GET | /rest/api/2/issue/{issueIdOrKey} | `fields=summary,status,assignee`; `expand=names,schema,renderedFields,transitions,changelog,operations,editmeta` (tùy endpoint/version). | Không có body. | `Accept: application/json` | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/PROJ-123?fields=summary,status,assignee,description" | `{"id":"10000","key":"PROJ-123","fields":{"summary":"Lỗi đăng nhập","status":{"name":"Open"},"assignee":{"name":"alice"}}}` |
| Sửa issue | PUT | /rest/api/2/issue/{issueIdOrKey} | `notifyUsers=true\|false` có thể có tùy cấu hình/patch; nên kiểm thử trên instance. | `fields` để đặt giá trị; `update` để add/set/remove theo operation. | JSON | 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"fields":{"summary":"Lỗi đăng nhập - cập nhật","labels":["api","urgent"]}}' "$JIRA_BASE/rest/api/2/issue/PROJ-123" | Không có body. |
| Xóa issue | DELETE | /rest/api/2/issue/{issueIdOrKey} | `deleteSubtasks=true\|false` nếu issue có sub-task. | Không có body. | `Accept: application/json` | 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X DELETE -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/PROJ-123?deleteSubtasks=true" | Không có body. |
| Gán assignee | PUT | /rest/api/2/issue/{issueIdOrKey}/assignee | Path: issue. | `{"name":"alice"}`; dùng `{"name":null}` để automatic assignee nếu project cho phép; chuỗi rỗng có thể dùng để unassign tùy cấu hình. | JSON | 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"alice"}' "$JIRA_BASE/rest/api/2/issue/PROJ-123/assignee" | Không có body. |
| Tìm issue bằng JQL (GET) | GET | /rest/api/2/search | `jql`; `startAt` mặc định 0; `maxResults` mặc định 50 và bị giới hạn bởi server; `validateQuery`; `fields`; `expand`. | Không có body. | `Accept: application/json` | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/search?jql=project%20%3D%20PROJ%20AND%20status%20%3D%20Open&startAt=0&maxResults=50&fields=summary,status,assignee" | `{"startAt":0,"maxResults":50,"total":1,"issues":[{"id":"10000","key":"PROJ-123","fields":{"summary":"..."}}]}` |
| Tìm issue bằng JQL (POST) | POST | /rest/api/2/search | Dùng khi JQL/field list dài. | `{"jql":"project = PROJ ORDER BY created DESC","startAt":0,"maxResults":50,"fields":["summary","status","assignee"]}` | JSON | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"jql":"project = PROJ ORDER BY created DESC","startAt":0,"maxResults":50,"fields":["summary","status","assignee"]}' "$JIRA_BASE/rest/api/2/search" | `{"startAt":0,"maxResults":50,"total":125,"issues":[...]}` |

### Ghi chú field Jira

| Vấn đề | Cách xử lý |
| --- | --- |
| Field không xuất hiện hoặc báo “cannot be set” | Kiểm tra Create/Edit Screen và dùng `createmeta` hoặc `editmeta`. |
| Custom field | Gửi bằng ID như `customfield_10000`; không dựa vào display name. |
| User field | Jira 6.4.x chủ yếu dùng username (`name`), không dùng Atlassian Account ID của Cloud. |
| Option field | Thường gửi `{"id":"10001"}` hoặc `{"value":"Option A"}` tùy schema trả về trong metadata. |
| Date vs datetime | Date: `yyyy-MM-dd`; datetime thường gồm milliseconds và offset `+0700`. |
| Null / clear field | Dùng `fields: {"field": null}` hoặc `update` operation theo schema của field. |

## 8. Jira — Comment, transition, attachment, worklog, watcher

| Mục đích | Method | Path | Params | Request / headers | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Liệt kê comment | GET | /rest/api/2/issue/{issueIdOrKey}/comment | `startAt`, `maxResults`, `orderBy` nếu patch hỗ trợ. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/PROJ-123/comment" | `{"startAt":0,"maxResults":50,"total":1,"comments":[{"id":"10010","body":"Đã xử lý"}]}` |
| Thêm comment | POST | /rest/api/2/issue/{issueIdOrKey}/comment | `expand` tùy chọn. | `{"body":"Đã xác nhận lỗi trên môi trường test.","visibility":{"type":"role","value":"Developers"}}`; bỏ `visibility` để comment thường. | 201 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"body":"Đã xác nhận lỗi trên môi trường test."}' "$JIRA_BASE/rest/api/2/issue/PROJ-123/comment" | `{"id":"10010","body":"Đã xác nhận lỗi trên môi trường test.","author":{"name":"alice"}}` |
| Sửa comment | PUT | /rest/api/2/issue/{issueIdOrKey}/comment/{id} | `expand` tùy chọn. | `{"body":"Nội dung đã cập nhật"}` | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"body":"Nội dung đã cập nhật"}' "$JIRA_BASE/rest/api/2/issue/PROJ-123/comment/10010" | `{"id":"10010","body":"Nội dung đã cập nhật"}` |
| Xóa comment | DELETE | /rest/api/2/issue/{issueIdOrKey}/comment/{id} | — | — | 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X DELETE -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/PROJ-123/comment/10010" | Không có body. |
| Liệt kê transition | GET | /rest/api/2/issue/{issueIdOrKey}/transitions | `expand=transitions.fields` để biết field bắt buộc trên transition screen. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/PROJ-123/transitions?expand=transitions.fields" | `{"transitions":[{"id":"31","name":"In Progress","to":{"name":"In Progress"},"fields":{}}]}` |
| Thực hiện transition | POST | /rest/api/2/issue/{issueIdOrKey}/transitions | — | `{"transition":{"id":"31"},"fields":{"resolution":{"name":"Fixed"}},"update":{"comment":[{"add":{"body":"Chuyển trạng thái qua API"}}]}}`; chỉ gửi field hợp lệ. | 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"transition":{"id":"31"},"update":{"comment":[{"add":{"body":"Chuyển trạng thái qua API"}}]}}' "$JIRA_BASE/rest/api/2/issue/PROJ-123/transitions" | Không có body. |
| Upload attachment | POST | /rest/api/2/issue/{issueIdOrKey}/attachments | Multipart field bắt buộc tên `file`; có thể truyền nhiều `-F file=@...`. | `X-Atlassian-Token: nocheck`; `Content-Type` do curl tự sinh cùng boundary. | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "X-Atlassian-Token: nocheck" -F "file=@./error.log" "$JIRA_BASE/rest/api/2/issue/PROJ-123/attachments" | `[{"id":"10020","filename":"error.log","mimeType":"text/plain","size":1234,"content":".../attachment/10020/error.log"}]` |
| Xóa attachment | DELETE | /rest/api/2/attachment/{id} | Path là attachment ID, không phải issue key. | — | 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X DELETE -H "Accept: application/json" "$JIRA_BASE/rest/api/2/attachment/10020" | Không có body. |
| Liệt kê worklog | GET | /rest/api/2/issue/{issueIdOrKey}/worklog | — | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/PROJ-123/worklog" | `{"startAt":0,"maxResults":20,"total":1,"worklogs":[{"id":"10030","timeSpentSeconds":3600}]}` |
| Thêm worklog | POST | /rest/api/2/issue/{issueIdOrKey}/worklog | `adjustEstimate=new\|leave\|manual\|auto`; với `newEstimate`/`reduceBy` khi tương ứng. | `{"comment":"Điều tra log","started":"2016-07-28T09:00:00.000+0700","timeSpentSeconds":3600}` | 201 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"comment":"Điều tra log","started":"2016-07-28T09:00:00.000+0700","timeSpentSeconds":3600}' "$JIRA_BASE/rest/api/2/issue/PROJ-123/worklog?adjustEstimate=auto" | `{"id":"10030","timeSpent":"1h","timeSpentSeconds":3600,"issueId":"10000"}` |
| Watchers | GET / POST / DELETE | /rest/api/2/issue/{issueIdOrKey}/watchers | DELETE: `username`; POST body là JSON string username. | POST body mẫu: `"bob"`. | GET 200; POST/DELETE 204 | curl -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Content-Type: application/json" --data '"bob"' "$JIRA_BASE/rest/api/2/issue/PROJ-123/watchers" | `{"isWatching":true,"watchCount":2,"watchers":[{"name":"alice"},{"name":"bob"}]}` cho GET. |
| Vote / unvote | POST / DELETE | /rest/api/2/issue/{issueIdOrKey}/votes | Không có. | Không có body. | 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" "$JIRA_BASE/rest/api/2/issue/PROJ-123/votes" | Không có body. |
| Tạo issue link | POST | /rest/api/2/issueLink | — | `{"type":{"name":"Blocks"},"inwardIssue":{"key":"PROJ-123"},"outwardIssue":{"key":"PROJ-124"},"comment":{"body":"Liên kết qua API"}}` | 201 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"type":{"name":"Blocks"},"inwardIssue":{"key":"PROJ-123"},"outwardIssue":{"key":"PROJ-124"},"comment":{"body":"Liên kết qua API"}}' "$JIRA_BASE/rest/api/2/issueLink" | Thường không có body; kiểm tra `Location` nếu reverse proxy giữ header. |

## 9. Jira — Project, component, version

| Mục đích | Method | Path | Params | Request body | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Liệt kê project người dùng xem được | GET | /rest/api/2/project | `expand=description,lead,url,projectKeys` tùy phiên bản. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/project" | `[{"id":"10000","key":"PROJ","name":"Internal Project","self":".../project/10000"}]` |
| Đọc project | GET | /rest/api/2/project/{projectIdOrKey} | `expand=description,lead,url,projectKeys` tùy phiên bản. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/project/PROJ" | `{"id":"10000","key":"PROJ","name":"Internal Project","lead":{"name":"alice"}}` |
| Statuses theo project | GET | /rest/api/2/project/{projectIdOrKey}/statuses | — | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/project/PROJ/statuses" | `[{"id":"3","name":"Task","statuses":[{"id":"1","name":"Open"}]}]` |
| Components của project | GET | /rest/api/2/project/{projectIdOrKey}/components | — | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/project/PROJ/components" | `[{"id":"10001","name":"Backend","project":"PROJ","lead":{"name":"alice"}}]` |
| Tạo component | POST | /rest/api/2/component | — | `{"name":"Backend","description":"Backend services","leadUserName":"alice","assigneeType":"COMPONENT_LEAD","project":"PROJ"}` | 201 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"Backend","description":"Backend services","leadUserName":"alice","assigneeType":"COMPONENT_LEAD","project":"PROJ"}' "$JIRA_BASE/rest/api/2/component" | `{"id":"10001","name":"Backend","project":"PROJ"}` |
| Sửa / xóa component | PUT / DELETE | /rest/api/2/component/{id} | DELETE có thể dùng `moveIssuesTo=<componentId>`. | PUT: các field cần cập nhật. | PUT 200; DELETE 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"Backend Platform","description":"Platform backend"}' "$JIRA_BASE/rest/api/2/component/10001" | `{"id":"10001","name":"Backend Platform","project":"PROJ"}` |
| Versions của project | GET | /rest/api/2/project/{projectIdOrKey}/versions | — | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/project/PROJ/versions" | `[{"id":"10002","name":"1.0.0","released":false,"archived":false}]` |
| Tạo version | POST | /rest/api/2/version | — | `{"name":"1.0.0","description":"Release đầu tiên","project":"PROJ","released":false,"archived":false,"releaseDate":"2016-08-31"}` | 201 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"1.0.0","description":"Release đầu tiên","project":"PROJ","released":false,"archived":false,"releaseDate":"2016-08-31"}' "$JIRA_BASE/rest/api/2/version" | `{"id":"10002","name":"1.0.0","projectId":10000,"released":false}` |
| Sửa / xóa version | PUT / DELETE | /rest/api/2/version/{id} | DELETE có thể dùng `moveFixIssuesTo` và `moveAffectedIssuesTo`. | PUT: các field cần cập nhật. | PUT 200; DELETE 204 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"1.0.1","released":false}' "$JIRA_BASE/rest/api/2/version/10002" | `{"id":"10002","name":"1.0.1","released":false}` |

## 10. Jira — User & permission

| Mục đích | Method | Path | Params | Request body | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Người dùng hiện tại | GET | /rest/api/2/myself | `expand=groups,applicationRoles` không phải patch nào cũng có. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/myself" | `{"name":"alice","emailAddress":"alice@example.com","displayName":"Alice","active":true}` |
| Đọc user | GET | /rest/api/2/user | `username` bắt buộc; `key` không nên dùng cho bản Jira cũ nếu chưa xác minh. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/user?username=alice" | `{"name":"alice","displayName":"Alice","active":true,"groups":{"size":2}}` |
| Tìm user | GET | /rest/api/2/user/search | `username` (chuỗi tìm kiếm); `startAt`; `maxResults`; `includeActive`; `includeInactive` tùy patch. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/user/search?username=ali&startAt=0&maxResults=20" | `[{"name":"alice","displayName":"Alice","active":true}]` |
| Tìm assignee hợp lệ | GET | /rest/api/2/user/assignable/search | `project` hoặc `issueKey`; `username`; `startAt`; `maxResults`. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/user/assignable/search?project=PROJ&username=ali" | `[{"name":"alice","displayName":"Alice","active":true}]` |
| Quyền hiện tại | GET | /rest/api/2/mypermissions | `projectKey`, `projectId`, `issueKey`, `issueId`, `permissions=BROWSE_PROJECTS,CREATE_ISSUES`. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/mypermissions?projectKey=PROJ&permissions=BROWSE_PROJECTS,CREATE_ISSUES" | `{"permissions":{"BROWSE_PROJECTS":{"havePermission":true},"CREATE_ISSUES":{"havePermission":false}}}` |
| Danh sách group cho user | GET | /rest/api/2/user/groups | `username` bắt buộc. | — | 200 | curl --fail-with-body -sS -u "$JIRA_USER:$JIRA_PASSWORD" -H "Accept: application/json" "$JIRA_BASE/rest/api/2/user/groups?username=alice" | `[{"name":"jira-users"},{"name":"developers"}]` |

## 11. Xác thực Bitbucket

Bitbucket Server 5.10.2 hỗ trợ Basic Auth, OAuth, cookie/trusted apps; đồng thời **Personal Access Token (PAT)** đã có từ Bitbucket Server 5.5.

| Cơ chế | Header / curl | Khuyến nghị |
| --- | --- | --- |
| PAT dạng Bearer | `curl -H "Authorization: Bearer $BB_TOKEN" "$BB_BASE/rest/api/1.0/projects"` | Ưu tiên cho script/integration. Tạo một token riêng cho mỗi integration và giới hạn permission. |
| PAT qua Basic Auth | `curl -u "$BB_USER:$BB_TOKEN" "$BB_BASE/rest/api/1.0/projects"` | Dùng khi HTTP client chỉ hỗ trợ Basic. Token thay cho password. |
| Username/password Basic | `curl -u "$BB_USER:$BB_PASSWORD" "$BB_BASE/rest/api/1.0/projects"` | Chỉ dùng qua HTTPS; kém thuận tiện khi password đổi. |
| OAuth 1.0a | `Authorization: OAuth ...` | Dùng cho ứng dụng nhiều người dùng hoặc flow ủy quyền. Cần cấu hình OAuth consumer. |
| Browser cookie / Trusted Applications | Cookie hoặc trusted-app headers theo cấu hình | Chỉ nên dùng khi integration legacy đã tồn tại; không phụ thuộc cookie browser cho service mới. |

### PAT

- UI: `Manage account` → `Account settings` → `Personal access tokens`.
- Token không được vượt quyền của user tạo token; permission trên token tiếp tục giới hạn phạm vi thao tác.
- Có thể revoke token mà không cần đổi password của user.
- Mẫu:

```bash
curl --fail-with-body -sS \
  -H "Authorization: Bearer $BB_TOKEN" \
  -H "Accept: application/json" \
  "$BB_BASE/rest/api/1.0/projects?start=0&limit=25"
```

## 12. Quy ước chung Bitbucket

### URI

```text
{BB_BASE}/rest/{api-name}/{api-version}/{resource-path}
```

Core API trong tài liệu này dùng:

```text
/rest/api/1.0/...
```

Repository cá nhân có project key dạng `~username`; URL cần encode `~` chỉ khi client/proxy yêu cầu. Nhiều repository endpoint cũng có user-centric URL, nhưng project-centric URL dễ chuẩn hóa hơn cho integration.

### Pagination Bitbucket

```json
{
  "size": 25,
  "limit": 25,
  "isLastPage": false,
  "values": [],
  "start": 0,
  "nextPageStart": 25
}
```

| Field | Ý nghĩa |
| --- | --- |
| `start` | Offset/page cursor của request hiện tại. |
| `limit` | Limit server đã áp dụng; có thể nhỏ hơn giá trị client yêu cầu. |
| `size` | Số phần tử trả về trong page. |
| `isLastPage` | `true` nếu đã hết. |
| `nextPageStart` | Cursor phải dùng cho page kế tiếp. **Không tự tính `start + limit`** vì cursor không được bảo đảm liên tục. |

Pseudo-code:

```text
start = 0
loop:
    page = GET endpoint?start=start&limit=100
    consume(page.values)
    if page.isLastPage: break
    start = page.nextPageStart
```

### Error body Bitbucket

```json
{
  "errors": [
    {
      "context": "fieldName",
      "message": "A detailed validation error message.",
      "exceptionName": null
    }
  ]
}
```

| HTTP status | Ý nghĩa thường gặp |
| ---: | --- |
| 400 | Params/body không hợp lệ. |
| 401 | Chưa xác thực hoặc permission không đủ theo cách một số endpoint cũ biểu diễn. |
| 403 | Đã xác thực nhưng bị cấm. |
| 404 | Resource không tồn tại hoặc user không được nhìn thấy. |
| 405 | Sai method. |
| 409 | Conflict: version stale, PR state không phù hợp, tên trùng, merge veto/conflict. |
| 415 | Sai `Content-Type`. |

## 13. Bitbucket — Project & repository

| Mục đích | Method | Path | Params | Request body | Permission | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Liệt kê project | GET | /rest/api/1.0/projects | `name`; `permission`; `start`; `limit`. | — | PROJECT_VIEW | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects?start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"key":"PRJ","id":1,"name":"My Project","public":false,"type":"NORMAL"}],"start":0}` |
| Tạo project | POST | /rest/api/1.0/projects | — | `{"key":"PRJ","name":"My Project","description":"Internal project","avatar":"data:image/png;base64,..."}`; `avatar` tùy chọn. | PROJECT_CREATE | 201 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"key":"PRJ","name":"My Project","description":"Internal project"}' "$BB_BASE/rest/api/1.0/projects" | `{"key":"PRJ","id":1,"name":"My Project","description":"Internal project","public":false,"type":"NORMAL"}` |
| Đọc project | GET | /rest/api/1.0/projects/{projectKey} | Path: `projectKey`. | — | PROJECT_VIEW | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ" | `{"key":"PRJ","id":1,"name":"My Project","description":"Internal project","public":false,"type":"NORMAL"}` |
| Sửa project | PUT | /rest/api/1.0/projects/{projectKey} | — | `{"name":"New name","description":"New description"}` | PROJECT_ADMIN | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"New name","description":"New description"}' "$BB_BASE/rest/api/1.0/projects/PRJ" | `{"key":"PRJ","id":1,"name":"New name","description":"New description"}` |
| Xóa project | DELETE | /rest/api/1.0/projects/{projectKey} | Project phải thỏa điều kiện xóa của server. | — | PROJECT_ADMIN | 204 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X DELETE -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ" | Không có body. |
| Liệt kê repository | GET | /rest/api/1.0/projects/{projectKey}/repos | `start`; `limit`. | — | PROJECT_VIEW; kết quả lọc theo REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos?start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"slug":"my-repo","id":1,"name":"My repo","scmId":"git","state":"AVAILABLE"}],"start":0}` |
| Tạo repository | POST | /rest/api/1.0/projects/{projectKey}/repos | — | Theo REST 5.10.2, chỉ `name` và `scmId` được dùng khi tạo. | PROJECT_ADMIN | 201 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"My repo","scmId":"git"}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos" | `{"slug":"my-repo","id":1,"name":"My repo","scmId":"git","state":"AVAILABLE","project":{"key":"PRJ"}}` |
| Đọc repository | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug} | — | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo" | `{"slug":"my-repo","id":1,"name":"My repo","scmId":"git","state":"AVAILABLE","forkable":true,"project":{"key":"PRJ"}}` |
| Sửa repository | PUT | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug} | — | `{"name":"Renamed repo","description":"...","forkable":true}`; field hợp lệ phụ thuộc phiên bản. | REPO_ADMIN | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"Renamed repo","description":"Repository renamed"}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo" | `{"slug":"renamed-repo","id":1,"name":"Renamed repo","state":"AVAILABLE"}` |
| Xóa repository | DELETE | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug} | — | — | REPO_ADMIN | 202 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X DELETE -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo" | Thường không có body; xóa có thể được xử lý bất đồng bộ. |
| Tải archive | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/archive | `at`; `filename`; `format=zip\|tar\|tar.gz\|tgz`; lặp `path`; `prefix`. | Response là binary. | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -o repo.zip "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/archive?at=master&format=zip&prefix=my-repo" | `Content-Type: application/octet-stream` hoặc tar tương ứng. |

## 14. Bitbucket — Branch & tag

| Mục đích | Method | Path | Params | Request body | Permission | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Liệt kê branch | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches | `base`; `details`; `filterText`; `orderBy=ALPHABETICAL\|MODIFICATION`; `start`; `limit`. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/branches?filterText=feature&orderBy=MODIFICATION&start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"id":"refs/heads/feature/ABC","displayId":"feature/ABC","type":"BRANCH","latestCommit":"...","isDefault":false}],"start":0}` |
| Tạo branch | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches | — | `{"name":"feature/ABC","startPoint":"refs/heads/master","message":"Create feature branch"}`; `startPoint` có thể là ref hoặc commit. | REPO_WRITE | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"feature/ABC","startPoint":"refs/heads/master","message":"Create feature branch"}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/branches" | `{"id":"refs/heads/feature/ABC","displayId":"feature/ABC","type":"BRANCH","latestCommit":"..."}` |
| Đọc default branch | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches/default | — | — | REPO_READ | 200 hoặc 204 nếu repo rỗng | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/branches/default" | `{"id":"refs/heads/master","displayId":"master","type":"BRANCH","latestCommit":"...","isDefault":true}` |
| Đặt default branch | PUT | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches/default | — | `{"id":"refs/heads/main"}` | REPO_ADMIN | 204 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"id":"refs/heads/main"}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/branches/default" | Không có body. |
| Liệt kê tag | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/tags | `filterText`; `orderBy=ALPHABETICAL\|MODIFICATION`; `start`; `limit`. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/tags?filterText=release&start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"id":"release-2.0.0","type":"TAG","latestCommit":"..."}],"start":0}` |
| Tạo tag | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/tags | — | `{"name":"v1.0.0","startPoint":"refs/heads/master","message":"Release 1.0.0"}` | REPO_WRITE | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"v1.0.0","startPoint":"refs/heads/master","message":"Release 1.0.0"}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/tags" | `{"id":"v1.0.0","displayId":"refs/tags/v1.0.0","type":"TAG","latestCommit":"..."}` |
| Đọc tag | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/tags/{name} | Path `{name:.*}` cho phép tên chứa dấu `/`; cần URL encode. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/tags/v1.0.0" | `{"id":"v1.0.0","displayId":"refs/tags/v1.0.0","type":"TAG","latestCommit":"..."}` |

## 15. Bitbucket — Source, commit, compare & diff

| Mục đích | Method | Path | Params | Request body / response type | Permission | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Duyệt file / đọc nội dung | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/browse/{path} | `at`; `type`; `blame`; `noContent`; `start`; `limit`. Bỏ `{path}` để duyệt root. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/browse/src/main/App.java?at=refs%2Fheads%2Fmaster" | `{"size":1,"limit":25,"isLastPage":true,"start":0,"lines":[{"text":"public class App {}"}]}` |
| Đọc raw file | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/raw/{path} | `at`. | Response là nội dung file, không nhất thiết JSON. | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/raw/README.md?at=master" | Bytes/text của file. |
| Liệt kê đường dẫn file | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/files/{path} | `at`; `start`; `limit`. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/files/src?at=master&start=0&limit=100" | `{"size":2,"limit":100,"isLastPage":true,"values":["src/main/App.java","src/test/AppTest.java"],"start":0}` |
| Liệt kê commit | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits | `followRenames=false`; `ignoreMissing=false`; `merges=exclude\|include\|only`; `path`; `since` exclusive; `until` inclusive; `withCounts=false`; paging. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/commits?until=refs%2Fheads%2Fmaster&merges=include&start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"id":"abcdef...","displayId":"abcdef0","message":"Fix login","author":{"name":"Alice"},"parents":[]}],"start":0}` |
| Đọc commit | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId} | `path` tùy chọn ở một số patch. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/commits/abcdef0123456789" | `{"id":"abcdef...","displayId":"abcdef0","message":"Fix login","author":{"name":"Alice"},"parents":[...]}` |
| Changes giữa hai revision | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/changes | `since`; `until` bắt buộc theo ngữ cảnh; paging nhưng server có hard cap. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/changes?since=master~1&until=master&limit=100" | `{"size":1,"limit":100,"isLastPage":true,"values":[{"path":{"toString":"src/App.java"},"type":"MODIFY","nodeType":"FILE"}],"start":0}` |
| Compare commits | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/compare/commits | `from`; `to`; `fromRepo`; paging. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/compare/commits?from=feature%2FABC&to=master&start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"id":"...","message":"Feature work"}],"start":0}` |
| Compare file changes | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/compare/changes | `from`; `to`; `fromRepo`; paging. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/compare/changes?from=feature%2FABC&to=master&start=0&limit=100" | `{"size":1,"limit":100,"isLastPage":true,"values":[{"path":{"toString":"src/App.java"},"type":"MODIFY"}],"start":0}` |
| Diff file | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/diff/{path} | `since`; `until`; `srcPath`; `contextLines`; `whitespace`; server áp hard cap, kiểm tra cờ `truncated`. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/diff/src/App.java?since=master~1&until=master&contextLines=3" | `{"diffs":[{"source":{"toString":"src/App.java"},"destination":{"toString":"src/App.java"},"hunks":[...],"truncated":false}]}` |

## 16. Bitbucket — Pull request & code review

| Mục đích | Method | Path | Params | Request body | Permission | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Liệt kê pull request | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests | `direction=INCOMING\|OUTGOING`; `at=refs/heads/...`; `state=OPEN\|DECLINED\|MERGED\|ALL`; `order=OLDEST\|NEWEST`; `withAttributes`; `withProperties`; participant filters; paging. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests?state=OPEN&order=NEWEST&start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"id":101,"version":1,"title":"Fix login timeout","state":"OPEN","fromRef":{"id":"refs/heads/feature/LOGIN-123"},"toRef":{"id":"refs/heads/master"}}],"start":0}` |
| Tạo pull request | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests | Source/target có thể ở hai repo khác nhau nhưng phải cùng repository hierarchy. | `title`, `fromRef`, `toRef`; `description`, `reviewers` tùy chọn. | REPO_READ trên source và target | 201 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"title":"Fix login timeout","description":"Điều chỉnh timeout và xử lý retry.","fromRef":{"id":"refs/heads/feature/LOGIN-123","repository":{"slug":"my-repo","project":{"key":"PRJ"}}},"toRef":{"id":"refs/heads/master","repository":{"slug":"my-repo","project":{"key":"PRJ"}}},"reviewers":[{"user":{"name":"charlie"}}]}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests" | `{"id":101,"version":1,"title":"Fix login timeout","state":"OPEN","open":true,"closed":false,"fromRef":{...},"toRef":{...},"reviewers":[...]}` |
| Đọc pull request | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId} | — | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101" | `{"id":101,"version":1,"title":"Fix login timeout","state":"OPEN","author":{...},"reviewers":[...],"participants":[]}` |
| Cập nhật pull request | PUT | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId} | Phải gửi `version` hiện tại để optimistic locking. | `{"version":1,"title":"Fix login timeout v2","description":"Updated","reviewers":[{"user":{"name":"charlie"}}]}` | Tác giả PR hoặc quyền phù hợp | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"version":1,"title":"Fix login timeout v2","description":"Updated","reviewers":[{"user":{"name":"charlie"}}]}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101" | `{"id":101,"version":2,"title":"Fix login timeout v2","state":"OPEN",...}` |
| Xóa pull request | DELETE | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId} | Body phải chứa `version` hiện tại. | `{"version":1}` | Tác giả nếu server cho phép, hoặc REPO_ADMIN | 204 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X DELETE -H "Accept: application/json" -H "Content-Type: application/json" --data '{"version":1}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101" | Không có body. |
| Activities | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/activities | `fromId`; `fromType=COMMENT\|ACTIVITY`; paging. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/activities?start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"id":301,"action":"COMMENTED","comment":{"text":"Looks good"}}],"start":0}` |
| Commits của PR | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/commits | Paging. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/commits?start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"id":"...","message":"Fix login"}],"start":0}` |
| Changes của PR | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/changes | `changeScope=ALL\|UNREVIEWED\|RANGE`; `sinceId`; `untilId`; `withComments`; `limit`. Endpoint này có thể trả tối đa một page/hard cap. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/changes?changeScope=ALL&withComments=true&limit=100" | `{"size":1,"limit":100,"isLastPage":true,"values":[{"path":{"toString":"src/App.java"},"type":"MODIFY"}],"start":0}` |
| Diff của PR | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/diff/{path} | `contextLines`; `sinceId`; `srcPath`; `untilId`; `whitespace`; kiểm tra `truncated`. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/diff/src/App.java?contextLines=3" | `{"diffs":[{"hunks":[...],"truncated":false}]}` |
| Thêm comment PR | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/comments | Có thể thêm general comment, reply (`parent.id`) hoặc inline comment (`anchor`). | `{"text":"Cần bổ sung unit test."}` | REPO_READ | 201 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"text":"Cần bổ sung unit test."}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/comments" | `{"id":401,"version":0,"text":"Cần bổ sung unit test.","author":{"name":"alice"},"state":"OPEN"}` |
| Sửa / xóa comment PR | PUT / DELETE | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/comments/{commentId} | PUT/DELETE thường yêu cầu `version` của comment để tránh ghi đè. | PUT: `{"text":"Nội dung mới","version":0}`; DELETE query/body version theo endpoint thực tế. | Tác giả comment hoặc quyền phù hợp | PUT 200; DELETE 204 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"text":"Nội dung mới","version":0}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/comments/401" | `{"id":401,"version":1,"text":"Nội dung mới"}` |
| Kiểm tra mergeability | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge | — | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/merge" | `{"canMerge":false,"conflicted":true,"outcome":"CONFLICTED","vetoes":[...]}` |
| Merge pull request | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge | `version` là version hiện tại của PR; mặc định `-1` nhưng nên luôn truyền. | Không có JSON body bắt buộc trong REST 5.10.2. | REPO_WRITE | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/merge?version=1" | `{"id":101,"version":1,"state":"MERGED","open":false,"closed":true,...}` |
| Decline PR | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/decline | `version` hiện tại trong query. | — | Tác giả hoặc REPO_WRITE theo policy | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/decline?version=1" | `{"id":101,"version":1,"state":"DECLINED","open":false,"closed":true,...}` |
| Reopen PR | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/reopen | `version` hiện tại trong query. | — | Quyền phù hợp | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/reopen?version=1" | `{"id":101,"version":1,"state":"OPEN","open":true,"closed":false,...}` |
| Đổi trạng thái review | PUT | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/participants/{userSlug} | Khuyến nghị thay cho endpoint `/approve` đã deprecated từ 4.2. | `{"user":{"name":"alice"},"approved":true,"status":"APPROVED"}`; status: `UNAPPROVED\|NEEDS_WORK\|APPROVED`. | REPO_READ; current user không được là author | 201 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"user":{"name":"alice"},"approved":true,"status":"APPROVED"}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/participants/alice" | `{"user":{"name":"alice"},"role":"REVIEWER","approved":true,"status":"APPROVED","lastReviewedCommit":"..."}` |
| Watch / unwatch PR | POST / DELETE | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/watch | — | — | REPO_READ | 204 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/101/watch" | Không có body. |

### Optimistic locking của pull request

| Thao tác | Cần version? | Cách lấy |
| --- | --- | --- |
| Update PR | Có, trong JSON body | `GET .../pull-requests/{id}` → field `version`. |
| Delete PR | Có, body `{"version":n}` | Đọc PR ngay trước khi xóa. |
| Merge / decline / reopen | Có, query `?version=n` | Đọc PR ngay trước thao tác. |
| Update/delete comment | Thường có version của comment | Đọc comment/activity hiện tại. |

Khi nhận `409`, đọc lại resource, đánh giá thay đổi rồi retry có kiểm soát; không tự ghi đè version mới nếu business state đã thay đổi.

## 17. Bitbucket — Webhook & repository hook

| Mục đích | Method | Path | Params | Request body | Permission | Success | Sample curl | Sample response |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Liệt kê webhook | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/webhooks | `event` có thể lặp; `statistics=false`; paging. | — | REPO_ADMIN | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/webhooks?statistics=false&start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"id":10,"name":"CI","events":["repo:refs_changed","pr:opened"],"url":"https://ci.internal/hook","active":true}],"start":0}` |
| Tạo webhook | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/webhooks | — | `{"name":"CI","url":"https://ci.internal/hook","events":["repo:refs_changed","pr:opened"],"active":true,"configuration":{"secret":"change-me"}}` | REPO_ADMIN | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"CI","url":"https://ci.internal/hook","events":["repo:refs_changed","pr:opened"],"active":true,"configuration":{"secret":"change-me"}}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/webhooks" | `{"id":10,"name":"CI","events":["repo:refs_changed","pr:opened"],"url":"https://ci.internal/hook","active":true}` |
| Test kết nối webhook | POST | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/webhooks/test | `url` bắt buộc. | — | REPO_ADMIN | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X POST -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/webhooks/test?url=https%3A%2F%2Fci.internal%2Fhook" | Đối tượng kết quả test webhook hoặc lỗi chuẩn. |
| Đọc webhook | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/webhooks/{webhookId} | `statistics=false\|true`. | — | REPO_ADMIN | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/webhooks/10?statistics=true" | `{"id":10,"name":"CI","events":[...],"url":"https://ci.internal/hook","active":true}` |
| Sửa webhook | PUT | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/webhooks/{webhookId} | — | Gửi cấu hình webhook đầy đủ cần giữ lại. | REPO_ADMIN | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"name":"CI v2","url":"https://ci.internal/hook","events":["repo:refs_changed","pr:opened","pr:merged"],"active":true}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/webhooks/10" | `{"id":10,"name":"CI v2","events":[...],"url":"https://ci.internal/hook","active":true}` |
| Xóa webhook | DELETE | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/webhooks/{webhookId} | — | — | REPO_ADMIN | 204 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X DELETE -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/webhooks/10" | Không có body. |
| Liệt kê repository hooks | GET | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/settings/hooks | `type=PRE_RECEIVE\|POST_RECEIVE`; paging. | — | REPO_READ | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -H "Accept: application/json" "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/settings/hooks?start=0&limit=25" | `{"size":1,"limit":25,"isLastPage":true,"values":[{"details":{"key":"com.example:hook"},"enabled":true}],"start":0}` |
| Enable / disable hook | PUT / DELETE | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/settings/hooks/{hookKey}/enabled | Hook phải tồn tại và cấu hình hợp lệ. | PUT có thể cần body settings của plugin; nếu hook không cần settings thì body rỗng. | REPO_ADMIN | 200/204 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/settings/hooks/com.example%3Ahook/enabled" | Thông tin hook hoặc không có body, tùy hook. |
| Đọc / cập nhật hook settings | GET / PUT | /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/settings/hooks/{hookKey}/settings | Cấu trúc JSON do plugin cung cấp hook quyết định; giới hạn serialized settings 32 KB. | PUT: JSON settings của hook. | GET REPO_READ; PUT REPO_ADMIN | 200 | curl --fail-with-body -sS -H "Authorization: Bearer $BB_TOKEN" -X PUT -H "Accept: application/json" -H "Content-Type: application/json" --data '{"enabledBranches":["master"],"strict":true}' "$BB_BASE/rest/api/1.0/projects/PRJ/repos/my-repo/settings/hooks/com.example%3Ahook/settings" | `{"enabledBranches":["master"],"strict":true}` |

### Event webhook thường dùng

Tên event thực tế phải lấy từ UI/reference của instance vì plugin có thể bổ sung event.

| Nhóm | Event ID thường gặp |
| --- | --- |
| Repository refs | `repo:refs_changed` |
| Repository modified | `repo:modified` |
| Pull request | `pr:opened`, `pr:modified`, `pr:reviewer:updated`, `pr:reviewer:approved`, `pr:reviewer:unapproved`, `pr:merged`, `pr:declined`, `pr:deleted`, `pr:comment:added`, `pr:comment:edited`, `pr:comment:deleted` |

> Không đưa secret thật vào file cấu hình hoặc log. Kiểm tra cơ chế signature/secret mà webhook subsystem hoặc plugin của instance thực tế hỗ trợ.

## 18. Error handling, retry và kiểm thử

### Chính sách retry đề xuất

| Loại lỗi | Retry? | Xử lý |
| --- | --- | --- |
| Connect timeout / reset / 502 / 503 / 504 | Có | Exponential backoff + jitter, số lần giới hạn; method ghi dữ liệu chỉ retry khi có cơ chế idempotency/dedup. |
| 429 | Có nếu server/gateway trả | Tôn trọng `Retry-After`; các version cũ có thể không có header này. |
| 400 / 415 | Không | Sửa params, JSON hoặc `Content-Type`. |
| 401 | Không retry mù | Kiểm tra credential/token, expiry/revoke, CAPTCHA Jira, reverse proxy và clock OAuth. |
| 403 / 404 | Không | Kiểm tra permission và resource visibility. |
| 409 | Có điều kiện | Đọc lại version/state rồi quyết định; đặc biệt PR optimistic locking và merge veto. |
| 5xx sau POST | Cẩn thận | Trước khi gửi lại, tìm resource theo key/name hoặc kiểm tra trạng thái để tránh tạo trùng. |

### Smoke test tối thiểu

```bash
# Jira: xác minh base URL, auth và version
curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  "$JIRA_BASE/rest/api/2/serverInfo"

curl --fail-with-body -sS \
  -u "$JIRA_USER:$JIRA_PASSWORD" \
  -H 'Accept: application/json' \
  "$JIRA_BASE/rest/api/2/myself"

# Bitbucket: xác minh auth và pagination
curl --fail-with-body -sS \
  -H "Authorization: Bearer $BB_TOKEN" \
  -H 'Accept: application/json' \
  "$BB_BASE/rest/api/1.0/projects?start=0&limit=1"
```

### Debug request không lộ secret

```bash
# Hiện status/header nhưng không dùng -v nếu terminal/log có thể lộ Authorization.
curl -sS -D response-headers.txt -o response-body.json \
  -H "Authorization: Bearer $BB_TOKEN" \
  -H 'Accept: application/json' \
  "$BB_BASE/rest/api/1.0/projects?limit=1"

jq . response-body.json
```

### Checklist tích hợp production

| Hạng mục | Yêu cầu |
| --- | --- |
| TLS | Tin cậy CA nội bộ; không dùng `--insecure`. |
| Credential | Secret manager; không commit vào Git; rotate/revoke định kỳ. |
| Account | Service account riêng; quyền tối thiểu; không dùng tài khoản admin cá nhân. |
| Version | Pin `/rest/api/2` cho Jira và `/rest/api/1.0` cho Bitbucket. |
| Pagination | Jira dùng `startAt`; Bitbucket dùng đúng `nextPageStart`. |
| Timeouts | Đặt connect/read timeout và retry có giới hạn. |
| Audit | Log method, sanitized URL, status, duration, request ID; không log password/token/cookie. |
| Compatibility | Chạy contract/smoke tests trên đúng internal host trước rollout. |
| Proxy/context | Kiểm tra base URL đã gồm `/jira` hoặc `/bitbucket`; kiểm tra proxy không strip `Authorization`. |
| JSON | Dùng UTF-8 và JSON serializer; không nối JSON thủ công trong code ứng dụng. |

## 19. Nguồn chính thức

| Chủ đề | Nguồn |
| --- | --- |
| Jira REST API reference 6.4.13 | https://docs.atlassian.com/software/jira/docs/api/REST/6.4.13/ |
| Jira Java API 6.4.14 (xác nhận bản phát hành) | https://docs.atlassian.com/software/jira/docs/api/6.4.14/ |
| Jira 6.4.14 release notes | https://confluence.atlassian.com/jira064/jira-6-4-14-release-notes-834232021.html |
| Jira Basic authentication | https://developer.atlassian.com/server/jira/platform/jira-rest-api-example-basic-authentication-6291732/ |
| Jira cookie-based authentication | https://developer.atlassian.com/server/jira/platform/cookie-based-authentication/ |
| Jira OAuth authentication | https://developer.atlassian.com/server/jira/platform/jira-rest-api-example-oauth-authentication-6291692/ |
| Bitbucket Server REST reference 5.10.2 | https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html |
| Bitbucket Basic authentication example | https://developer.atlassian.com/server/bitbucket/how-tos/example-basic-authentication/ |
| Bitbucket Server 5.5 release notes — PAT introduced | https://confluence.atlassian.com/bitbucketserver/bitbucket-server-5-5-release-notes-938037662.html |
| Bitbucket Personal access tokens | https://confluence.atlassian.com/bitbucketserver050/personal-access-tokens-964970357.html |

---

## Appendix A — Mẫu shell helper

```bash
jira_get() {
  local path="$1"
  curl --fail-with-body -sS \
    --connect-timeout 5 \
    --max-time 60 \
    -u "$JIRA_USER:$JIRA_PASSWORD" \
    -H 'Accept: application/json' \
    "$JIRA_BASE$path"
}

jira_json() {
  local method="$1"
  local path="$2"
  local body="$3"
  curl --fail-with-body -sS \
    --connect-timeout 5 \
    --max-time 60 \
    -u "$JIRA_USER:$JIRA_PASSWORD" \
    -X "$method" \
    -H 'Accept: application/json' \
    -H 'Content-Type: application/json' \
    --data "$body" \
    "$JIRA_BASE$path"
}

bb_get() {
  local path="$1"
  curl --fail-with-body -sS \
    --connect-timeout 5 \
    --max-time 60 \
    -H "Authorization: Bearer $BB_TOKEN" \
    -H 'Accept: application/json' \
    "$BB_BASE$path"
}

bb_json() {
  local method="$1"
  local path="$2"
  local body="$3"
  curl --fail-with-body -sS \
    --connect-timeout 5 \
    --max-time 60 \
    -H "Authorization: Bearer $BB_TOKEN" \
    -X "$method" \
    -H 'Accept: application/json' \
    -H 'Content-Type: application/json' \
    --data "$body" \
    "$BB_BASE$path"
}
```

## Appendix B — Mẫu phân trang Bitbucket bằng Bash + jq

```bash
start=0
while :; do
  page="$(
    bb_get "/rest/api/1.0/projects/PRJ/repos/my-repo/commits?until=master&start=$start&limit=100"
  )"

  jq -c '.values[]' <<<"$page"

  if [[ "$(jq -r '.isLastPage' <<<"$page")" == "true" ]]; then
    break
  fi

  start="$(jq -r '.nextPageStart' <<<"$page")"
done
```

## Appendix C — Mẫu phân trang Jira JQL bằng Bash + jq

```bash
start_at=0
max_results=100

while :; do
  body="$(
    jq -nc \
      --arg jql 'project = PROJ ORDER BY created ASC' \
      --argjson startAt "$start_at" \
      --argjson maxResults "$max_results" \
      '{jql:$jql,startAt:$startAt,maxResults:$maxResults,fields:["summary","status"]}'
  )"

  page="$(jira_json POST '/rest/api/2/search' "$body")"
  jq -c '.issues[]' <<<"$page"

  count="$(jq '.issues | length' <<<"$page")"
  total="$(jq '.total // -1' <<<"$page")"
  start_at=$((start_at + count))

  [[ "$count" -eq 0 ]] && break
  [[ "$total" -ge 0 && "$start_at" -ge "$total" ]] && break
done
```
