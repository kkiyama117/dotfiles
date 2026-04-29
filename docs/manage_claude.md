# 複数 Claude Code セッション管理ガイド

作成日: 2026-04-29
対象: ローカル開発で 2〜複数の Claude Code セッションを並行運用したいケース
前提環境: Linux (Manjaro) / zsh + sheldon / tmux

---

## 1. Claude Code 標準機能（追加インストール不要）

### セッション管理コマンド
- `claude --resume` / `claude -c` — 直近のセッションを再開（会話履歴を引き継ぐ）
- `claude --continue --fork-session` — 現在の履歴を引き継ぎつつ**新しい session ID** を発行。複数ターミナルでの並行作業に最適
- `/resume` — セッション一覧から選択して再開

### Git Worktree（公式推奨）
ディレクトリ単位でセッションを完全分離する最も堅実な方法。
```bash
# フィーチャーブランチ用に独立したディレクトリを作成
git worktree add ../feature-x feature/x
cd ../feature-x
claude   # 新しいセッションが別ディレクトリで開始

# 終了時
cd ..
git worktree remove feature-x
```
利点:
- ディレクトリ単位でセッションが独立 (`.claude/projects/` の session ID も自動分離)
- branch / working tree が物理的に分かれるため context 競合が起きない

### バックグラウンド実行
```bash
claude "task description" &
```
Bash tool 内では `run_in_background` パラメータで tool-level のバックグラウンド化も可能。

---

## 2. ECC（Everything Claude Code）スキル

`settings.json` に登録済みのスキルから、複数セッション運用に直接効くもの:

| スキル | 用途 | 適規模 |
|---|---|---|
| `/dmux-workflows` | tmux ペイン管理で複数エージェント並行（`n` で追加・`m` で結果統合） | 3〜5 並行 |
| `/claude-devfleet` | DAG（有向非環グラフ）計画 → 隔離 worktree → parallel dispatch | 大規模・複数機能同時開発 |
| `/autonomous-agent-harness` | persistent memory + scheduling で完全自動化 | 長時間ループ |
| `/autonomous-loops` | quality gate / recovery 付き連続ループ | 反復タスク |

### 使用例
```text
# tmux ベースで軽量並行
claude "/dmux-workflows"
# セッション内: n → "Fix auth bug" / n → "Optimize DB queries" / m で統合

# 計画駆動で大規模並行
claude "/claude-devfleet"
# "Refactor API layer, add caching, update tests, migrate DB"
# → 自動 DAG 計画 → 複数 worktree で parallel dispatch
```

---

## 3. 推奨セットアップ（ソロ開発 2〜4 セッション）

最小コストで最高信頼性のセットアップは **Git Worktree + `--fork-session` + tmux ウィンドウ分割**。

### tmux + worktree パターン
```bash
# tmux ウィンドウ 1: フィーチャー A
tmux new-window -n claude-a
git worktree add ../feat-a feature/a
cd ../feat-a
claude --fork-session "Build feature A"

# tmux ウィンドウ 2: フィーチャー B
tmux new-window -n claude-b
git worktree add ../feat-b feature/b
cd ../feat-b
claude --fork-session "Build feature B"

# tmux ウィンドウ 3: テスト・リファクタ（main 留置）
tmux new-window -n claude-test
cd /path/to/main
claude --fork-session "Run full test suite"

# 後から再開
tmux attach -t <session>
```

### スケール判断
| 状況 | 推奨アプローチ |
|---|---|
| 2〜4 個の独立タスク | Git Worktree + `--fork-session` + tmux |
| 5 個前後の軽量並行 | `/dmux-workflows` |
| 計画駆動の大規模並行 | `/claude-devfleet` |
| 長時間自動ループ | `/autonomous-agent-harness` / `/autonomous-loops` |

---

## 4. 注意点

- `BW_SESSION` 等の機密 env は worktree 横断で継承されるため、不要セッションでは `bw_lock` で破棄する（CLAUDE.md 参照）
- `chezmoi apply` 対象のリポジトリ（`~/.local/share/chezmoi`）で worktree を切るときは、`.chezmoiignore` の更新影響範囲に注意
- 並行セッション同士で同じファイルを編集しないようタスク分割すること（git worktree でも working tree が同じ branch を指すと衝突する）

---

## 5. 公式ドキュメント

- セッション管理: <https://code.claude.com/docs/en/how-claude-code-works.md>
- Agent Teams: <https://code.claude.com/docs/en/agent-teams>
- Common workflows（worktree 含む）: <https://code.claude.com/docs/en/common-workflows>
- ECC スキル一覧: <https://github.com/affaan-m/everything-claude-code>
