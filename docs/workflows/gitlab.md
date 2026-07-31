# Reviewing a GitLab merge request

Requires [`glab`](https://gitlab.com/gitlab-org/cli) authenticated with
`glab auth login`. The adapter is chosen by host, so the TUI is identical to
the GitHub flow — the title bar badge reads `glab`.

## Opening a merge request

```bash
leanreview 'https://gitlab.com/group/repo/-/merge_requests/42'
leanreview 'group/repo!42'                # nested subgroups work too
leanreview 42                             # inside a checkout with a GitLab origin
```

GitLab's changes endpoint returns per-file hunks; leanreview reconstructs the
canonical unified patch from them, so comment positions align with what
GitLab shows.

Everything from the [GitHub workflow](github.md) applies: the `p` details
overlay, `◆` thread markers, `Enter` to read a thread, `r` to stage a reply,
`C` for the comment list.

## How submission maps onto GitLab

GitLab has no atomic review-with-comments endpoint, so `s` maps the
submission onto GitLab's model:

- each draft line comment becomes a **positioned diff discussion**;
- the review summary becomes a merge-request **note**;
- **Approve** approves the MR;
- **Request changes** posts a "Changes requested" note.

Comments post in order, and a mid-way failure reports how many were already
published — those are cleared from your drafts so a retry cannot repost them.
