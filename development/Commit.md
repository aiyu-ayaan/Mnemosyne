# Commit Guidelines

This file contains the commit message guidelines for the project. Following these guidelines will help maintain a clear and consistent history of changes, making it easier for team members to understand the purpose of each commit.

---

## ⚠️ CRITICAL RULE FOR AI ASSISTANTS: NEVER PUSH CODE

> **AI assistants must NEVER execute `git push` or push code/commits to remote repositories (`origin/master`, `origin/main`, or any remote target) under any circumstances.**
>
> All commits must remain strictly **LOCAL** on the user's machine. Pushing commits to remote repositories is exclusively the manual decision and responsibility of the human repository owner.

---

## Commit Message Structure

A commit message should consist of a short summary, a detailed description (if necessary), and any relevant metadata. The structure is as follows:

```
<type>(<scope>): <short summary>
```

- You can add a detailed description after the short summary if needed to provide additional context about the changes made in the commit.
- After each commit, update the documentation in `@devdocs` (`development/devdocs/`) and `docs/docs/` with the relevant information.
- When committing changes that touch the `development/` submodule, commits must be created in both the submodule repo and the main repo.

---

## The Sub-Repo Is a Git Submodule

`development/` is a separate git repository (`BetweenUs-Dev-Docs`) checked out as a submodule inside this repository.

### Avoiding Detached HEAD in Submodules

1. **Detached HEAD**: `git submodule update` and a plain `cd development` check out a commit, not a branch. Committing there commits onto nothing — the commit exists in git reflog, but no branch points to it.
2. **Before committing inside `development/` (or any submodule), every time:**

   ```bash
   cd development
   git branch --show-current   # empty output means detached HEAD - stop
   ```

   If the output is empty: run `git checkout main` (or `git switch -c main origin/main` if `main` does not exist locally yet) **before** creating any commit. Never commit on a detached HEAD.

3. **Local commits for both repositories:**
   - Commit changes inside `development/` on branch `main`.
   - Update the submodule pointer in the root repository and commit in the main repo on branch `master`.
   - **REMINDER**: DO NOT run `git push`. Leave all commits local.

---

## Commit Types

The following commit types are recommended for use in this project:

- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, missing semi-colons, etc.)
- `refactor`: Code refactoring without changing functionality
- `test`: Adding or updating tests
- `chore`: Maintenance tasks (build process, package manager, etc.)
- `perf`: Performance improvements
- `ci`: Continuous integration changes
- `revert`: Reverting a previous commit
- `build`: Changes that affect the build system or external dependencies

---

## Scope

The scope of a commit message should indicate the area of the codebase that is affected by the changes (e.g., `backend`, `desktop`). If the scope is not applicable, it can be omitted.

---

## Short Summary

The short summary should be a concise description of the changes made in the commit:

- Written in the imperative mood (e.g., "Add feature" instead of "Added feature").
- Keep it concise and focused (recommended under 72 characters).
- Provides enough information to understand the purpose of the commit at a glance.

---

## Release & Documentation Trigger Commits

Automated releases and documentation deployments are triggered via commit markers on `master` parsed by `.github/workflows/release.yml` and `.github/workflows/docs.yml`:

- `!alpha`, `!beta`, `!stable`, `!fix`, `!feat`, `!major`, `!patch`: Trigger the automated release pipeline.
- `!docs`: Triggers immediate Docusaurus site build and deploy to `gh-pages`.

For the complete guide on markers, platform scopes (`docker`, `desktop`, `android`, `docs`), and the autonomous procedure for AI assistants when asked to cut a release (*"alpha release done"*), see [`development/Release-Commit.md`](Release-Commit.md).

---

## Important Considerations for AI Agents

1. **NEVER Push to Remote**: Never run `git push`, `git push origin`, or any push variant. Commits are strictly local.
2. **No AI Attribution**: Never add yourself as the author or co-author of the commit message. The commit message should reflect the repository username, never an AI assistant's name or co-authored-by tag.
3. **Keep Commits Atomic**: Break distinct changes into smaller, logical commits with clear messages.
