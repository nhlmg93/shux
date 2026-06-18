# AGENTS.md

## Project Description

Building shux to replace tmux with a simpler, modern, reliable architecture.

## Development Guidelines

### Documentation Boundaries
- The `docs/` directory is the **Astro Starlight** end-user documentation site (markdown in `docs/src/content/docs/`).
- Published docs: [https://nhlmg93.github.io/](https://nhlmg93.github.io/) via GitHub Pages (`nhlmg93.github.io` repo).
- The GitHub wiki is a lightweight hub for links and community supplements only; canonical content lives in `docs/`.
- Do not treat `docs/` as a place for internal developer notes, implementation scratch docs, or agent workflow notes.
- Developer and agent notes belong in `AGENTS.md`, not in `docs/`.


### Testing

#### Time budgets (hard limits per test function)

| Layer | Path | Max per test |
| --- | --- | --- |
| Unit | `./internal/...` | **2 seconds** |
| Integration / sim / e2e | `./test/integration/...`, `./test/sim/...`, `./test/e2e/...` | **5 seconds** |

- Every test must bound itself with `context.WithTimeout(t.Context(), …)` (or equivalent waits) inside these limits.
- `go test -timeout` in the Makefile is a **package** safety net (sum of tests in the package), not the per-test budget.
- Prefer fast, layered tests: pure replay/manifest checks in `internal/`, actor/daemon flows in `test/integration/`, user-story sims in Docker (`make test-sim`).

#### What to test (no trivial unit tests)

Integration-style tests should be focused on Sessions/Window/Pane/Config/Plugin and shux-specific behavior:
    * Interactions around the recovery model
    * Internal messaging
    * Persistence/Durability layers
    * Lua Configuration/Extension(plugins)
    * Session Recovery/Resurrection
- E2E/Regression/Deterministic Testing 
    * Use Docker Container as a test bed.
        - Contains testing dependencies(Less, nano, node)
    * User Story based. Heavily targeted towards what make shux unique
        - Recovery model.
        - Persistence/Durability layers
        - Session Recovery/Resurrection
        - Ex: As a user i have shux running 4 panes. In top left i have less with documentation. Top right some kind of long running 
              Node process. Bottom left Nano editor where i edit text. Bottom right a plain terminal i use to run shell commands. After detaching shux I expect to be able to reattach as close to the state i left it in as possible.
        - Sim tests must exercise **live** behavior (real shells, real commands, real journals). Do not add tests that only check binary presence or pre-seed manifest/journal files to skip the live path.
    * Fuzz/Stress Testing is how we will ensure shuxs Durability/Reliablity as a running process.
        - At the end of the day our Recovery/Resurrection and Persistence/Durability model/layer will only ever be as good as the shu  x process is reliable.
        - Tiger Style Programming

### Programming Style
    - Tiger Style
        * The Power of "No" (Minimalism & Control)
            - Boundedness: Try to eliminate any source of unpredictability at runtime.
        * Assertions as Documentation and Defense
            - Checks to ensure state is exactly what the programmer expects it to be.
        * Total Transparency and Observability
            - Static Analysis: Lean heavily on the compiler to catch errors.
            - Deterministic Simulation Testing: code is structured so that it can be run inside a 
              Deterministic Sim(Docker container as test bed)
    - Erlang/Elixir/BEAM 
        * "Let It Crash" (Fault Tolerance)
        * Concurrency via the Actor Model
        * Pre-emptive Scheduling (Soft Real-Time)

### Git Workflow
- Only commit/push when user explicitly says so
- Always run full test suite and ci check before pushing.
- Write thoughtful but brief commit messages describing what the commit introduces

## External Reference Checkouts

When you need upstream implementation/reference material, these repositories should be available under `/tmp`:
- `/tmp/tmux` -> `https://github.com/tmux/tmux.git`
- `/tmp/libghostty` -> `https://github.com/mitchellh/go-libghostty.git` (Go libghostty bindings)
- `/tmp/bubbletea` -> `https://github.com/charmbracelet/bubbletea.git`
- `/tmp/lipgloss` -> `https://github.com/charmbracelet/lipgloss.git`
- `/tmp/pty` -> `https://github.com/creack/pty.git`

If any of these are missing, clone them into `/tmp` before relying on them for reference.
