# Top-Level Rules

- To maximize efficiency, **if you need to execute multiple independent processes, invoke those tools concurrently, not sequentially**.
- **You must think exclusively in English**. However, you are required to **respond in Japanese**.

# Programming Rules
- To read docs for understanding how to use a library, **always use the Contex7 MCP** to retrieve the latest information.

## Knowledge Management System

This repository implements a systematic knowledge management approach for Claude Code sessions. The `(PROJECT_ROOT)/.claude/` directory structure helps maintain project context, technical insights, and operational patterns across different sessions.

1.  First, create a plan and document the requirements in `(PROJECT_ROOT)/.claude/tasks/design.md`.
2.  Read  `Claude.md` and `(PROJECT_ROOT)/.claude/project-knowledge.md` in project, and update design if needed.
2.  Based on the requirements, identify all necessary tasks, number thm, and list them in a Markdown file at `(PROJECT_ROOT).context/tasks/todos.md`.
3.  Once the plan is established, create a new branch and begin your work.
    - Branch names should start with `feature/` followed by a brief summary of the task.
4.  Break down tasks into small, manageable units that can be completed within a single commit.
    - Always update `.context/tasks/todos.md` to mark how far a task has been completed
5.  Create a checklist for each task to manage its progress.
6.  Always apply a code formatter to maintain readability.
7.  If it takes a long time to resolve the issue or if there are project-specific issues, please add them to `(PROJECT_ROOT)/.claude/project-knowledge.md`.
8.  Do not commit your changes. Instead, ask for confirmation.
9.  When instructed to create a Pull Request (PR), use the following format:
    - **Title**: A brief summary of the task.
    - **Key Changes**: Describe the changes, points of caution, etc.
    - **Testing**: Specify which tests passed, which tests were added, and clearly state how to run the tests.
    - **Related Tasks**: Provide links or numbers for related tasks.
    - **Other**: Include any other special notes or relevant information.

## Tools 

We need to use `mise` to manage programming languages except `Python` and `Rust`.  We also use `mise` for shorthand aliases.
If you need to learn how to use `mise`, read `~/.config/claude/docs/tools/mise.md`

