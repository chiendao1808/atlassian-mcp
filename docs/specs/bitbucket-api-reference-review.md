# Bitbucket API Reference Review — v1.4

## 1. Review scope

The v1.3 implementation plan and bundled endpoint table were checked tool-by-tool against the bundled reference and the official Bitbucket Server 5.10.2 REST document: https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html

The result is implemented in `bitbucket-tool-implementation-guide.md`; this review records the changes and the remaining host-specific gates.

## 2. Material corrections and additions

| Area | v1.3 gap | v1.4 contract |
|---|---|---|
| Branch ordering | Allowed values not stated | `ALPHABETICAL` or `MODIFICATION` only |
| Default branch | `204` behavior easy to parse as JSON | Typed empty-repository success; no decoder call |
| Commit listing | `ignoreMissing` omitted | All documented filters included, including `ignoreMissing` |
| Commit identity | Ref/commit ambiguity | Commit GET passes commit ID/SHA unchanged; no branch normalization |
| Commit changes | Described as ordinary paging | `since`, `withComments`, server hard cap; no promise of later content |
| Commit diff | Missing `autoSrcPath`, `since`, `withComments` | Full query contract and nested truncation preservation |
| Compare upstream repo | Raw `fromRepo` could broaden scope | MCP exposes only `fromRepositorySlug` in configured project |
| File commit | Multipart fields/status not fully operationalized | Exact multipart fields; 200 success; one PUT; stale-write guard |
| PR list participants | Filter serialization not specified | Continuous `username.N`, optional `role.N`/`approved.N`, maximum 10 |
| PR activities | Cursor dependency missing | `fromType` required when `fromId` is present |
| PR changes | Generic pagination wording | One-page result; `start` ignored; internal/request limit cap |
| PR diff | Generic response limit wording | Non-paged hard-cap; preserve upstream truncation separately |
| PR comments | One generic anchor object | Separate general, reply, file, and line payload validation |
| Review status | URL/body identity mismatch unresolved | Fixed configured identity plus mandatory staging compatibility gate |
| PR transitions | `409` risked flattening | Preserve conflict/veto/stale/invalid-state distinctions; no replay |

## 3. Source-link strategy

The bundled API reference now contains Section 20 with one stable anchor per MCP tool. Each guide section links to that anchor and also names the exact heading in the official 5.10.2 document. This keeps offline planning usable while allowing an implementer to verify the original resource.

## 4. Explicit stop conditions

Implementation must stop for specification review when:

- the official and bundled references disagree on a request field or success response;
- the target host returns an identity contract incompatible with `BITBUCKET_USER_SLUG` for participant update;
- a response cap cannot be distinguished from MCP-layer truncation;
- the target host exposes a response envelope that the guide marks as a staging gate;
- an implementation would require a new caller-supplied project, URL, header, user identity, or retry behavior.

## 5. Required evidence

For every tool, keep request-recording contract fixtures and MCP schema snapshots. For staging gates, store sanitized method/path/query/status/content-type and structural JSON shape, never tokens, raw file content, comment text, or diff lines.
