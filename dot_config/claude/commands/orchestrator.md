---
description: Split complex tasks into sequential steps, where each step can contain multiple parallel subtasks.
---

# Claude Code Orchestrator

Split complex tasks into sequential steps, where each step can contain multiple parallel subtasks.

## Workflow Pattern of the orchestrator process

1. **Initial Analysis**
    - If you can choose `Claude Opus` model, use it when doing this section. 
    - Identify key components and dependencies.
    - Plan sequential steps and parallel subtasks
    - You can use browser if the information about tools or tasks is needed.
    - Make subfolder for this orchestrator task in `.claude/current_task` 
      - We call this folder for the task `task_debug_folder`.
    - Write the plan down at `(task_debug_folder)/design.md`
2. **Step Planning**
    - If you can choise `Claude Opus` model, use it when doing this section. 
    - Based on the requirements, identify all necessary task.
      - Design a hierarchical structure consisting of a major goal (step) and subtasks necessary to achieve it.
      - Each step can contain multiple parallel subtasks
    - Define what context from previous steps is needed for each step
    - Create `steps planning` section in `(task_debug_folder)/todos.md` and write about each step and subtask.
      - each task should be numbered and checkbox list. 
3. **Step-by-Step Execution**
    - Once the plan is established, create a new git branch and begin your work.
      - Branch names should start with `feature/` followed by a brief summary of the task.
    - If you can choise `Claude Sonnet` model, use it when doing this section (especially in coding) . 
    - Break down tasks into small, manageable units that can be completed within a single commit.
    - Execute subtasks within a step (in parallel if planed in `todos.md`).
      - Always update `.context/tasks/todos.md` to mark how far a task has been completed
    - Create a checklist for each task to manage its progress.
    - Wait for all subtasks in current step to complete
    - Pass relevant results to next step
4. **Step Review and Adaptation**
    - If you can choise `Claude Opus` model, use it when doing this section. 
    - After each step completion, review results
    - Validate if remaining steps are still appropriate
    - Adjust next steps based on discoveries
    - Add, remove, or modify subtasks at `todos.md` and `plan.md` if needed.
    - Request concise summaries (100-200 words) from each subtask for message of git commit.
    - Do not commit your changes by yourself. Instead, ask for confirmation when each step is done.
5. ** Execution and step loop **
    - If you can choise `Claude Opus` model, use it when doing this section. 
    - Once one step is complete, repeat steps 3-5 of this list for the next step, and repeat until all tasks are complete.
      - Synthesize results from the completed step
      - Use synthesized results as context for next step
      - Build comprehensive understanding progressively
    - Maintain flexibility to adapt plan
      - And also update `(task_debug_folder)/plan.md` and `step_be_step.md`, `(task_debug_folder)/repomix-output.xml`.
6. **Finish task**
    - At last (when all task completed), move `(task_debug_folder)` into `.claude/archive/(git branch name)`.
    - When instructed to create a Pull Request (PR), use the following format:
      - **Title**: A brief summary of the task.
      - **Key Changes**: Describe the changes, points of caution, etc.
      - **Testing**: Specify which tests passed, which tests were added, and clearly state how to run the tests.
      - **Related Tasks**: Provide links or numbers for related tasks.
      - **Other**: Include any other special notes or relevant information.

## Best Practices

### Task Decomposition
- Break complex tasks into 2-4 logical sequential steps
- Identify opportunities for parallel execution within steps
- Maintain clear dependencies between steps

### Context Management
- Use concise summaries when passing context between steps
- Avoid context overflow by focusing on essential information
- Re-evaluate and adapt plans after each step completion

### Execution Strategy
- Start with comprehensive project/task analysis
- Execute independent subtasks in parallel for efficiency
- Continuously validate progress and adjust approach
- Minimize manual intervention through automation

## Adaptive Planning

The orchestrator supports dynamic workflow adaptation:
- **Progressive Understanding**: Each step builds on previous discoveries
- **Plan Modification**: Workflow can be adjusted based on findings
- **Error Recovery**: Automatic handling of common development issues
- **Context Optimization**: Efficient information passing between steps

## Key Benefits

- **Sequential Logic**: Steps execute in order, allowing later steps to use earlier results
- **Parallel Efficiency**: Within each step, independent tasks run simultaneously
- **Memory Optimization**: Each subtask gets minimal context, preventing overflow
- **Progressive Understanding**: Build knowledge incrementally across steps
- **Clear Dependencies**: Explicit flow from analysis → execution → validation

## Implementation Notes

- Always start with a single analysis task to understand the full scope
- Group related parallel tasks within the same step
- Pass only essential findings between steps (summaries, not full output)
- Use TodoWrite to track both steps and subtasks for visibility
- After each step, explicitly reconsider the plan:
    - Are the next steps still relevant?
    - Did we discover something that requires new tasks?
    - Can we skip or simplify upcoming steps?
    - Should we add new validation steps?

## Adaptive Planning Example

```
Initial Plan: Step 1 → Step 2 → Step 3 → Step 4

After Step 2: "No errors found in tests or linting"
Adapted Plan: Step 1 → Step 2 → Skip Step 3 → Simplified Step 4 (just commit)

After Step 2: "Found critical architectural issue"
Adapted Plan: Step 1 → Step 2 → New Step 2.5 (analyze architecture) → Modified Step 3
```

### Performance Optimization
- Group related operations within steps
- Minimize file system operations across parallel tasks
- Use efficient tool configurations
- Cache results when appropriate
