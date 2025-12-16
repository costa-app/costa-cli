# Contributing to costa-cli

Thank you for your interest in contributing to costa-cli! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.25 or later
- [just](https://just.systems/) (optional, for task running)
- [mise](https://mise.jdx.dev/) (optional, for version management)

### Getting Started

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/YOUR-USERNAME/costa-cli.git
   cd costa-cli
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests to verify your setup:
   ```bash
   go test ./...
   # or
   just test
   ```

## Making Changes

### Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages. While individual commits in your PR don't strictly need to follow this format (since we squash-merge), it's good practice for clear history.

Format: `type(scope): description`

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Test changes
- `chore`: Maintenance tasks
- `ci`: CI/CD changes
- `perf`: Performance improvements
- `build`: Build system changes
- `revert`: Revert a previous commit
- `security`: Security fixes

**Examples:**
```
feat(auth): add OAuth2 PKCE flow support
fix(setup): prevent overwriting custom Claude Code settings
docs: update installation instructions
test(cli): add tests for login command
```

### Pull Request Title

**Important:** The PR title MUST follow the Conventional Commits format. This is enforced by CI and is required because we use squash-merge - GitHub will use your PR title as the commit message on the main branch.

Good PR titles:
- `feat: add support for custom OAuth providers`
- `fix(token): handle expired token edge case`
- `docs: improve setup command examples`

Bad PR titles:
- `Update auth.go`
- `Fixed bug`
- `WIP: adding new feature`

### Pull Request Process

1. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```

2. **Make your changes** and commit them with descriptive commit messages

3. **Run all checks** before pushing:
   ```bash
   just ci
   ```
   This runs:
   - Code formatting (`go fmt`)
   - Vet (`go vet`)
   - Module tidying (`go mod tidy`)
   - Linting (`golangci-lint`)
   - Tests (`go test`)
   - Vulnerability scanning (`govulncheck`)

4. **Push to your fork** and create a pull request

5. **Ensure CI passes**: Our CI runs two independent workflows:
   - **lint**: Checks code quality, formatting, and PR title format
   - **test**: Runs the test suite

   Both must pass before your PR can be merged.

6. **Address review feedback** if any

### Code Style

- Follow standard Go conventions and idioms
- Run `go fmt` before committing
- Ensure `golangci-lint run` passes with no warnings
- Write clear, descriptive variable and function names
- Add comments for non-obvious logic

### Testing

- Write tests for new features and bug fixes
- Ensure all tests pass: `go test ./...`
- Aim for meaningful test coverage of critical paths
- Use table-driven tests where appropriate

## Squash-Merge Policy

We use **squash-merge** for all pull requests. This means:

- All commits in your PR will be squashed into a single commit on `main`
- The squashed commit message will be your **PR title** (followed by the PR number)
- Your individual commit messages won't appear in the main branch history
- This keeps the main branch history clean and readable

**Why this matters:**
- Your PR title becomes part of the permanent project history
- The PR title is what shows up in changelogs and release notes
- That's why we enforce Conventional Commits format on PR titles

## Branch Protection and Release Process

### Branch Protection

The `main` branch is protected:
- Direct pushes are not allowed
- All changes must go through pull requests
- Required status checks must pass (lint + test workflows)
- Only maintainers can merge pull requests

### Releases

Releases are restricted to maintainers only and are automated via GitHub Actions:

1. Maintainer creates and pushes a tag: `just release v1.2.3`
2. GitHub Actions automatically:
   - Runs tests
   - Verifies the tag is on the main branch (for stable releases)
   - Builds universal macOS binaries
   - Signs and notarizes the binary
   - Creates a GitHub release with changelog
   - Updates Homebrew taps

**Tag Protection:**
- Only maintainers can create tags matching `v*`
- This prevents unauthorized releases

**Prerelease Tags:**
- Prerelease versions (e.g., `v1.2.3-beta.1`) can be created from any branch
- Stable versions (e.g., `v1.2.3`) must be created from the `main` branch

## Questions or Issues?

- Open an issue for bugs or feature requests
- Start a discussion for questions or ideas
- Check existing issues and PRs before creating a new one

## License

By contributing, you agree that your contributions will be licensed under the BSD 3-Clause License.
