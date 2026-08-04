#!/usr/bin/env node
/*
 * PreToolUse hook for the `Agent` tool.
 *
 * Rule: an agent may NOT spawn another agent of the SAME function (same agent
 * type) — that just clones the caller while it idles and waits. Spawning a
 * DIFFERENT agent type (per the caller's instructions) is allowed, and the main
 * thread may spawn anything, including several agents of the same type.
 *
 * Enforcement: deny only when the calling subagent's own `agent_type` equals the
 * requested `subagent_type`. The main thread has no `agent_type`, so it is never
 * blocked.
 *
 * Configured in .claude/settings.json under hooks.PreToolUse (matcher: "Agent").
 */
let raw = "";
process.stdin.on("data", d => (raw += d));
process.stdin.on("end", () => {
  let input = {};
  try { input = JSON.parse(raw || "{}"); } catch { process.exit(0); }

  const tool = input.tool_name || input.toolName;
  if (tool !== "Agent") process.exit(0); // only guards Agent spawns

  const caller = input.agent_type || input.agentType;      // undefined on main thread
  const ti = input.tool_input || input.toolInput || {};
  const target = ti.subagent_type || ti.agent_type;

  if (caller && target && caller === target) {
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: {
        hookEventName: "PreToolUse",
        permissionDecision: "deny",
        permissionDecisionReason:
          `Blocked: a '${caller}' agent may not spawn another '${caller}' agent ` +
          `(no same-function recursion — the parent would just idle and wait). ` +
          `Do this work yourself, or spawn a different agent type as your instructions require.`
      }
    }));
    process.exit(0);
  }
  process.exit(0); // allow everything else
});
