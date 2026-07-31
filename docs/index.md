# leanreview

A terminal code-review client. It reviews a **patch/diff file**, a **local git
comparison**, a **GitHub pull request**, or a **GitLab merge request** in the
same TUI: navigate the diff, attach draft comments anchored to semantic diff
locations, then **export them as Markdown** (great for feeding review notes
back to an AI as prompt feedback) or **submit them as a real review**.

It is a review client, not a git client: the installed `git`, `gh`, and `glab`
handle repository and forge semantics; leanreview owns navigation, rendering,
review state, and comments.

![The main view](screens/main.svg)

*The main view: changed-files sidebar, unified diff, a draft comment in its
inline box (`●`), and an existing review thread marker (`◆`).*

## Install

```bash
go install github.com/perrito666/leanreview/cmd/leanreview@latest
```

Or from a checkout:

```bash
make            # builds ./leanreview
make install    # installs into GOBIN
```

Per-mode optional dependencies: [`gh`](https://cli.github.com/) (after
`gh auth login`) for GitHub, [`glab`](https://gitlab.com/gitlab-org/cli)
(after `glab auth login`) for GitLab. Patch-file and local-git review need
nothing but `git` — and a plain patch file needs not even that.

## Quick start

```bash
leanreview change.diff        # review a patch file
leanreview --base main        # review this branch against main
leanreview 418                # review PR/MR 418 of this repo's origin
leanreview --list             # pick something from your review queue
```

## Where to go next

- **[Reviewing without a forge](workflows/local.md)** — patch files, stdin,
  working tree, staged changes, revision ranges, and Markdown export.
- **[Reviewing a GitHub pull request](workflows/github.md)** — threads,
  replies, and atomic review submission through `gh`.
- **[Reviewing a GitLab merge request](workflows/gitlab.md)** — the same flow
  through `glab`, and how submission maps onto GitLab's model.
- **[Discovering what to review](workflows/discovery.md)** — `--list`,
  engine filters, and named filters.
- **[Concepts](concepts.md)** — how drafts, anchoring, and relocation work.
- **[Keys](reference/keys.md)**, **[Command line](reference/cli.md)**, and
  **[Configuration](reference/configuration.md)** reference.
