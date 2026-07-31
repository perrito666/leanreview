package ui

// HelpText returns the key reference shown in the help overlay.
func HelpText() string {
	return `leanreview — keys

 Navigation
   j / k, ↓ / ↑  down / up one line
   J / K         next / previous change
   ]c / [c       next / previous hunk
   Tab / S-Tab   next / previous file (also ]f / [f)
   gg / G        first / last line
   Ctrl-d/u      half page down / up
   PgDn / PgUp   full page down / up
   h / l, ← / →  scroll (unified) or target side (split)
   0 / $         scroll to line start / end

 View
   t             toggle unified / split
   T             toggle full file context around the diff
   S             cycle syntax colors (red/green · everywhere · off)
   w             toggle line / comment wrapping
   i             toggle inline comment previews
   \             toggle changed-files sidebar
   za            fold / unfold current hunk
   zR / zM       expand / collapse all hunks
   /             search diff text
   n / N         next / previous match
   f             file picker
   C             comment list
   Enter         open the conversation / thread on a commented line

 Review
   v             start line selection
   V             select current changed block
   c             comment on line / selection
   e             edit draft comment under cursor
   x             dismiss / restore comment under cursor
   r             reply to the comment / thread under cursor
   dd            delete comment under cursor

 Pull-request mode
   p             PR details (title, description, link)
   s             submit review (confirmation screen)
   :comment      submit as a general comment
   :approve      submit approving
   :request      submit requesting changes

 Commands
   :w            save drafts
   :export FILE  export comments (.json: review exchange, else Markdown)
   :q            quit
   ?             toggle this help

 Press ?, esc, or q to close.`
}
