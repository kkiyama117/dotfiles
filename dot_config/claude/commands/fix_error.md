---
description: Fix error with linting and testing tool
---


1. Read `(PROJECT_ROOT)/CLAUDE.md` and get info and rule in this repository.
2. Run test command and list all error up. and write them to `(PROJECT_ROOT)/.claude/fix-errors.md`. Errors must be numbered without overlap.
3. Fix errors and write about what you changed and how to fix and so on. Rule of this is written in `CLAUDE.md`. When starting to fix each error, add the error number to be fixed to the end of `.claude/fix-errors.md`. If the number already exists, first check whether the error has been resolved. If the error has not been fixed, resolve that error first.
4. If the error is resolved, in addition to the steps specified in Claude.md, delete the corresponding error from the error list in `.claude/fix-errors.md`.
5. If you fix all error, run test and check it has no error.
6. If you find error by test, do the same as written above of this routine (2~5). If we got no error, skip them and go the next item in this bulleted list.
6. Please refer to the instructions in Claude.md, and update files and `git commit` as necessary.