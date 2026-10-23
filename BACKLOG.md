# Backlog

Items to build later, in rough priority order.

## Panel editing

- Edit the environment name in place (click the name).
- Pick the color by clicking its dot, instead of a dropdown.
- Expand an environment to see its keys, and add, edit, or delete values from the panel.

## Layering at runtime

- Apply the base `.env` plus the active environment when debugging (F5) and when running commands.
- Decide: (A) base + active, which is the current plan, or (B) pick several environments as a stack, in order.

## Branches and storage

- Show the environment panel grouped by branch, with the current branch first.
- Let any branch's environment be activated, so it can be used while on another branch.
- Show the branch name in the status bar.
- Refresh automatically when the branch changes.

## Later

- Gitignore guard for env files that are still inside the project.
- Swap mode (replace `.env` with the active file, with backup and restore).
- Delete-to-trash instead of permanent deletion.
