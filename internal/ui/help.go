package ui

// HelpText returns the key reference shown in the help overlay.
func HelpText() string {
	return `leanreview — keys

 Navigation
   j / k         down / up one line
   J / K         next / previous change
   ]c / [c       next / previous hunk
   ]f / [f       next / previous file
   gg / G        first / last line
   Ctrl-d/u      half page down / up

 View
   t             toggle unified / split
   \             toggle changed-files sidebar
   za            fold / unfold current hunk
   zR / zM       expand / collapse all hunks
   /             search diff text
   n / N         next / previous match
   f             file picker
   C / Enter     comment list

 Review
   v             start line selection
   V             select current changed block
   c             comment on line / selection
   e             edit draft comment under cursor
   dd            delete comment under cursor

 Pull-request mode
   r             reply to the thread under the cursor
   s             submit review (confirmation screen)
   :comment      submit as a general comment
   :approve      submit approving
   :request      submit requesting changes

 Commands
   :w            save drafts
   :export FILE  export comments as Markdown
   :q            quit
   ?             toggle this help

 Press ?, esc, or q to close.`
}
