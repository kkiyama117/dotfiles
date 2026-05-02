---
description: Tear down the current claude-managed tmux window and its git worktree (slash-command equivalent of tmux prefix + C + k).
argument-hint: (no args)
---

# /branch-finish — 現 worktree + tmux window の破棄

`/branch-out` で生やした worktree + window を片付けるための後始末コマンド。
tmux キーバインド `prefix + C + k` (`claude_table k`) と同一の `claude-kill-session`
バイナリを叩く。

## 重要な制約

- **このコマンドが走るとカレント tmux window が消え、それに伴い現 Claude
  セッションも終了する**。差し戻し不可の破壊的操作。
- **`claude-kill-session` は `git worktree remove --force` を発行するため、
  worktree 内の uncommit な変更・untracked ファイル・stash されていない
  作業はすべて消える**。実行前に commit / push を済ませておくこと。
  `/branch-finish` は単に worktree と window を解体するだけで、コミット・
  push・PR 起票は行わない (それらは `/commit-push-pr` 等の他コマンドで対応する)。
- worktree 削除が何らかの理由で失敗した場合 (lock 等) も、tmux window kill は
  独立に実行される。worktree が孤立して残り、window だけ消える。
- **あなた (現セッションの Claude) は破棄判断を再考・延期してはならない**。
  ユーザがこのコマンドを発話した時点で意図は確定している。確認プロンプトは
  返さない。

## 入力

引数なし。`$ARGUMENTS` は空のはず。何か渡された場合も無視する。

## 手順

### 1. 前提チェック (1 秒以内)

- `TMUX` 環境変数が空 → tmux 外なので `/branch-finish` は無効。1 行で
  「tmux 外では実行不可」と返して終了。
- それ以外 (tmux 内) → 何もせず手順 2 へ。

### 2. claude-kill-session を起動 (Bash 1 回のみ)

```bash
~/.local/bin/claude-kill-session
```

- バイナリ内蔵の 3-stage safety check
  (`@claude-managed=yes` / pane が `claude` を実行中 / session 名が
  `claude-` prefix) がいずれか満たされなければバイナリ側が exit 1 で失敗し、
  tmux popup でエラー文が表示される。Claude セッションは生き残る。
- safety check を通過した場合、worktree 削除 → window kill が走り、
  この Claude セッションは window と共に終了する。**Bash の戻り値は
  返ってこない可能性が高い**。

### 3. 報告 (戻ってきた場合のみ)

破棄が拒否された (= safety check 失敗) ケースのみ、stderr の内容を 1 行で
そのまま見せる。成功時は window が消えて報告する場が無くなるため、
追加メッセージは不要。

## 補足

- 確認プロンプトは tmux キーバインド側の `confirm-before` で実現していたが、
  スラッシュコマンドはタイプミスでは起動しないため不要。
- 同等の手動操作は tmux で `prefix + C + k` → `y`。
- `claude-kill-session` は内部で `git worktree remove --force` を呼ぶため、
  uncommitted 変更があると **問答無用で破棄される**。残しておきたい変更が
  あれば `/branch-finish` の前に `git stash` か `git commit` で確保しておく。
