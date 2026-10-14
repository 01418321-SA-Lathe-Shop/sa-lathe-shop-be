# Mandatory phase commit and push workflow

Applies to every phase. Current authorized branch: `develop`.

## During implementation

- Decide logical groups before coding, then commit each completed group as work
  progresses. Do not wait to make one catch-all phase commit.
- Each commit explains one purpose and normally contains 3-8 related files.
  Keep cohesive changes together; do not add unrelated files to meet a count.
- Order dependencies so every committed snapshot builds independently. Include
  relevant tests. Run `go build ./...` before each commit and focused tests.
- Use clear Conventional Commits describing what changed and why. The history
  should make failures easy to trace, review, revert or bisect.

Example groups (adapt to the phase; not a requirement to invent these changes):

| Group | Example message |
|---|---|
| Domain rules and unit tests | `feat: validate quotation pricing inputs` |
| Schema/repository and tests | `feat: persist quotation revisions atomically` |
| Routing and HTTP tests | `feat: expose authorized quotation endpoints` |
| Setup and verification docs | `docs: document quotation API and test results` |

Avoid `update`, `changes`, `done`, or combining unrelated fixes/features. A small
documentation-only follow-up can be one focused commit; do not artificially
split it. Never squash a multi-feature phase into one commit before pushing.

## At the end of every completed phase

1. Update PLAN.md with completed items, decisions and any remaining limitations.
2. Run full build, vet, tests and relevant integration/Docker checks.
3. Inspect the staged diff, `git status` and `git log --oneline`; exclude secrets,
   `.env`, local data, output and unrelated user edits.
4. Push the phase's separate commits to the authorized remote/branch. Do not
   accumulate several completed phases for one final project-wide push.
5. Verify the remote tip. Report branch, commits, verification and limitations.

One end-of-phase push may carry multiple already-separated logical commits;
the prohibited pattern is one giant catch-all commit for all the phase's work.
Do not push `main`, force-push or rewrite published commits without explicit
authorization. If remote access or approval blocks pushing, preserve local
commits and report the blocker and pending commits honestly.
