---
description: Rebase the current worktree onto a target branch (main/develop/etc.) and merge it back. Pass `squash` as a second word for a squash merge.
argument-hint: <target_branch> [squash] [no-rebase] [fetch]
---

# /branch-merge — 現 worktree を target branch に統合

`/branch-out` で生やした worktree のコミットを `main` / `develop` 等の
統合先ブランチに取り込むコマンド。`~/.local/bin/claude-branch-merge` が
3 ステップ (target worktree 検索 → rebase → merge) を 1 発で実行する。

## 重要な制約

- target branch は **既に別 worktree として check out されている必要がある**。
  例えば `main` は `~/.local/share/chezmoi/` で check out 済み。`develop`
  が無ければ先に `claude-tmux-new develop --no-claude` 等で worktree を
  生やしておく。
- rebase が conflict した場合バイナリは exit 1 で abort する。conflict は
  ユーザが手動で解決する必要がある (`/branch-merge` は再試行を自動化しない)。
- merge も同様 (rebase 通過後は通常 fast-forward で conflict は出ないはず)。
- **あなた (現セッションの Claude) は target branch 名を勝手に推測してはならない**。
  ユーザが明示しなかった場合のみ「main か develop か」を 1 行で確認する。

## 入力解析

`$ARGUMENTS` を空白区切りで分解し、最初のトークンを target_branch として扱う。
2 番目以降のトークンに以下のキーワードが含まれていれば対応するフラグに
変換する (順序不問・大文字小文字不問):

| キーワード | バイナリへのフラグ |
|---|---|
| `squash` | `--squash` |
| `no-rebase` / `norebase` | `--no-rebase` |
| `fetch` | `--fetch` |

例:
- `/branch-merge main` → `claude-branch-merge main`
- `/branch-merge develop squash` → `claude-branch-merge develop --squash`
- `/branch-merge main fetch squash` → `claude-branch-merge main --fetch --squash`

## 手順

### 1. 引数チェック (1 秒以内)

- `$ARGUMENTS` が空 → 1 行で「target_branch を指定してください (例: `/branch-merge main`)」と返して終了。
- target_branch が `main` / `develop` 以外 (例: 別 feature ブランチを target にしようとしている) → そのまま実行する。バイナリが妥当性を判定する。

### 2. claude-branch-merge を起動 (Bash 1 回)

```bash
~/.local/bin/claude-branch-merge '<TARGET>' [<FLAGS>]
```

- `<TARGET>`: 手順 1 で抽出したブランチ名。single-quote で囲う。
- `<FLAGS>`: 上の対応表で変換したフラグ群。
- バイナリは成功時に `merged <source> into <target> at <path>` (squash 時は
  `squash-merged ...`) を stdout に 1 行出力。
- 失敗時 (rebase conflict / merge conflict / target worktree 未存在 等) は
  stderr にエラー詳細を出して exit 1。

### 3. 報告 (1 行)

成功したら、バイナリの stdout 1 行をそのまま見せる。

> `merged feat/x into main at /home/kiyama/.local/share/chezmoi`

失敗したら、stderr の最終行をそのまま見せる。conflict の場合はそれ以上の
解決策を提案せず、ユーザの手動対応に委ねる。

## 補足

- バイナリは `git -C <target_worktree> merge` を呼ぶため、merge commit や
  squash commit は **target worktree 側のローカル HEAD に積まれる**。
  `git push` は別途必要。
- `--squash` 時の commit メッセージは `Squash merge <source> into <target>` +
  `git log --oneline <target>..<source>` の各行を `* ...` で列挙する形に
  自動生成される。エディタは開かない。
- `--fetch` を付けると `git fetch origin` を target worktree で先に走らせる。
  CI などで target が remote に追従していない疑いがあるときに使う。
- merge 後の片付け (worktree 削除 + tmux window 終了) は `/branch-finish` を
  別途呼ぶ。`/branch-merge` は片付けまで自動化しない。
