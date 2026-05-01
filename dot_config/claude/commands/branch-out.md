---
description: Hand off a task to a fresh Claude session in a new git worktree + tmux window. Lightweight branch-name derivation only, no analysis.
argument-hint: <task description>
---

# /branch-out — 新規 worktree + tmux window への タスクハンドオフ

ユーザの依頼内容を、新しい git worktree + tmux window 上の **fresh Claude セッション** に渡すための spawn コマンド。

## 重要な制約

- **あなた (現セッションの Claude) は依頼内容を分析・設計・実装してはならない**。深く考えるのは新セッションの仕事。
- 現セッションの仕事は (a) ブランチ名生成、(b) spawn コマンド実行、(c) 1行報告 のみ。
- 既存ファイルの探索、計画立案、実装プレビューはすべて NG。

## 入力

ユーザの依頼内容 (1行〜複数行):

```
$ARGUMENTS
```

## 手順

### 1. ブランチ名を生成 (深く考えない)

形式: `<type>/<kebab-summary>`

- `type`: `feat` / `fix` / `refactor` / `docs` / `chore` のいずれか。message の動詞から1秒で推定。
  - "plan", "add", "implement", "create", "design" → `feat`
  - "fix", "bug", "error", "broken" → `fix`
  - "refactor", "cleanup", "simplify", "consolidate" → `refactor`
  - "doc", "document", "readme" → `docs`
  - 雑務 (deps, formatting) → `chore`
  - 不明 → `feat`
- `kebab-summary`: 2〜4語、英小文字、ハイフン区切り、最大 30 字、`[a-z0-9-]` のみ。
  - 日本語入力の場合も意味を汲んで英 kebab にする (ローマ字音訳ではなく意訳)。

例:
- "Plan a worktree spawner feature" → `feat/worktree-spawner`
- "Fix the dispatcher race condition" → `fix/dispatcher-race`
- "Refactor tmux scripts for clarity" → `refactor/tmux-scripts`
- "新しいログ収集機能を設計したい" → `feat/log-collector`

### 2. spawn を起動 (Bash 1 回のみ)

```bash
~/.config/tmux/scripts/tmux-claude-new.sh '<BRANCH>' \
  --worktree-base "${XDG_DATA_HOME:-$HOME/.local/share}/worktrees" \
  --prompt '<MESSAGE>'
```

- `<BRANCH>`: 手順1で生成したブランチ名。single-quote で囲う。
- `<MESSAGE>`: `$ARGUMENTS` 全文。single-quote で囲い、内部の `'` は `'\''` にエスケープする。
- 失敗時は stderr メッセージをそのまま見せる (リトライ・回避策の自動模索はしない)。

### 3. 1 行報告

成功したら:

> spawn 完了: `<branch>` (worktree: `<path>`)。tmux window へフォーカス切替済み。

それ以上の説明 (新セッションでの実装方針、ブランチ名選定理由など) は **書かない**。

## 補足

- worktree 配置: `~/.local/share/worktrees/<repo>/<sanitized-branch>` (XDG 中央集約。slash はハイフンに sanitize される)。
- 同名ブランチが既に worktree として存在する場合は `tmux-claude-new.sh` が既存 path を再利用 (新規作成はしない)。
- 新セッションは `claude '<MESSAGE>'` で起動するため、最初のメッセージとして依頼内容が pre-fill される。
- `--no-claude` 相当 (claude 抜きの worktree のみ作成) は本コマンドではサポートしない。必要なら直接 `tmux-claude-new.sh <branch> --no-claude --worktree-base ...` を叩く。
