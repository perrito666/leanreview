" Syntax for leanreview review-exchange files (*.review.json).
" JSON base, plus diff colors for the embedded patch: the canonical format
" puts one diff line per JSON array element on its own physical line, so the
" whole-line matches below are reliable against leanreview's own output.
if exists('b:current_syntax')
  finish
endif

" Base JSON highlighting.
runtime! syntax/json.vim
unlet! b:current_syntax

" Diff lines inside the "patch" array. Anchored to whole lines that are a
" single JSON string element; ordered so the more specific header matches win.
syn match lreviewDiffFile    /^\s*"diff --git .\{-}",\?$/
syn match lreviewDiffHeader  /^\s*"\(---\|+++\) .\{-}",\?$/
syn match lreviewDiffHunk    /^\s*"@@ .\{-}",\?$/
syn match lreviewDiffAdded   /^\s*"+\([+"]\)\@!.\{-}",\?$/
syn match lreviewDiffRemoved /^\s*"-\([-"]\)\@!.\{-}",\?$/

hi def link lreviewDiffFile    diffFile
hi def link lreviewDiffHeader  diffFile
hi def link lreviewDiffHunk    diffLine
hi def link lreviewDiffAdded   diffAdded
hi def link lreviewDiffRemoved diffRemoved

let b:current_syntax = 'lreview'
