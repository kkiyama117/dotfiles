# lazygit — 機能と利点まとめ

## lazygit とは

Jesse Duffield 氏が Go で開発した **Git 用ターミナル UI (TUI)**。2026年4月時点で **GitHub スター 76,000+、8年継続開発、最新 v0.61.0**。MIT ライセンス。

> *"git is powerful, but what good is that power when everything is so damn hard to do?"* — README

---

## 解決する根本問題（モチベーション）

| 生 git CLI の痛み | lazygit の解 |
|---|---|
| インタラクティブ rebase で TODO ファイルを手編集 | `i` → `s/f/d/e/Ctrl+k/j` の単一キー操作 |
| 部分ステージング (`git add -p`) が煩雑 | `space` で行単位、`v` で範囲、`a` でハンク |
| 過去コミットへの修正が複雑 (rebase + amend + push) | `Shift+A` で背後で自動 rebase amend |
| 不要な stash 強要 | `w` で worktree を作って並行作業 |

**定量効果**: 利用者は **1日 15〜20 分の節約**を体感（fixup squash や論理的並び替えを「ためらわず」できるため）。

---

## 主要機能カタログ

### 1. 6 パネル UI（一画面完結）
Status / Files / Branches / Commits / Stash / Diff を同時表示。タブ切替で Submodules・Worktrees・Reflog にもアクセス。

### 2. ステージング & コミット
- `space` 行単位 / `v` 範囲 / `a` ハンク全体
- `Shift+A` 過去コミット amend（自動 rebase）
- `z` / `Shift+Z` で **undo/redo**（reflog ベース）

### 3. インタラクティブ rebase「Magic」
- `i` で開始、`s` squash / `f` fixup / `d` drop / `e` edit
- `Ctrl+k` / `Ctrl+j` でコミット並べ替え
- `Shift+B` で base コミットマーク → `r` で rebase
- **カスタムパッチ** (`Ctrl+P`)：古いコミットから特定行を抜き出して別コミットへ移動

### 4. ブランチ・PR
- `/` フィルタ、`Enter` でコミット表示
- `gh` CLI 連携で **PR ステータスアイコン**を branch 名に表示
- `Shift+G` でブラウザに PR を開く
- gitflow があれば `i` キーで自動サポート

### 5. Worktrees（重要）
`w` で作成 → stash や WIP コミット不要で複数ブランチ並行作業。Claude Code × tmux ワークフローと相性◎

### 6. Cherry-pick / 比較 / Bisect
- `Shift+C` コピー → `Shift+V` ペースト
- `Shift+W` で 2 コミット diff モード
- `b` で bisect の good/bad マーク

### 7. リセット & クリーンアップ
- `Shift+D` "nuke" — dirty submodule も含めて全削除
- ファイル単位の discard / restore も対話的

### 8. カスタムコマンド（拡張性の核）
`config.yml` に YAML で定義。プロンプト・選択メニュー・現在選択中のコミット/ブランチ/ファイルを変数として受け取れる:

```yaml
customCommands:
  - key: 'C'
    command: 'git commit -m "{{.Form.Message}}"'
    prompts:
      - type: 'input'
        title: 'Commit message'
        key: 'Message'
```

→ shuntaka 氏の **AI commit (`!aic`)** はこの仕組みで実装。

### 9. カスタムページャ
delta / diff-so-fancy / difftastic を組み込んで diff を任意整形可能。

---

## 競合 TUI / GUI との比較

| ツール | 言語 | 強み | 弱み |
|---|---|---|---|
| **lazygit** | Go | 学習コスト最小・rebase magic・カスタムコマンド・PR 連携 | 巨大リポ（nixpkgs クラス）で `git status` 待ちで UI 凍結 |
| **gitui** | Rust | 大規模リポでも高速、深いカスタマイズ | 学習コスト高、機能はやや限定 |
| **tig** | C | 軽量、ハンクステージは得意 | 機能数が少ない |
| **magit** | Emacs Lisp | コミット時に message buffer + diff を同時表示する最強 UX | Emacs への commitment 必須 |
| **GitHub Desktop / IDE 内蔵** | 各種 | GUI で見やすい | キーボード完結できない、rebase 等の高度操作が貧弱 |

**選び方の指針**:
- 初学者・大半のリポ → **lazygit**
- nixpkgs / monorepo クラスの巨大リポ → **gitui**
- Emacs 民 → **magit**
- 「ハンクステージだけほしい」→ **tig**

---

## 統合・エコシステム

- **GitHub CLI (`gh`)**: PR 表示・ブラウザオープン
- **外部エディタ**: rebase TODO 編集等
- **tmux popup**: `tmux display-popup -E lazygit` で全画面 TUI として呼び出すのが定番（m3tech / shuntaka 両記事）
- **Neovim 連携**: toggleterm.nvim や lazygit.nvim プラグイン
- **Submodules / Worktrees**: ネイティブ対応
- **delta / difftastic**: ページャ統合

---

## 既知の制限

1. **巨大リポでの遅さ** — `git status` が 10 秒+ かかるリポでは UI が固まる（最大の弱点）
2. **カスタマイズの粒度**は gitui より浅い（簡素さとのトレードオフ）

---

## 主要インストール手段（Manjaro 環境向け）

```bash
sudo pacman -S lazygit          # 公式リポ
paru -S lazygit-git             # AUR 開発版
```

他: brew / apt (Ubuntu 25.10+) / dnf (copr) / scoop / winget / nix / conda / `go install`

---

## この dotfiles リポへの示唆

- 既に **paru でインストール済み**かは要確認（`run_once_all_os.sh.cmd.tmpl` への追加候補）
- `dot_config/lazygit/config.yml` を chezmoi 管理にすると、**delta 統合・カスタムコマンド・キーバインド**が再現可能
- shuntaka 氏の **`!aic` AI commit** や m3tech の **`prefix + g` popup** を chezmoi 化すれば tmux/Claude ワークフローと統合できる

---

## 参考リンク

- [GitHub - jesseduffield/lazygit](https://github.com/jesseduffield/lazygit)
- [Lazygit in 2026: The Git TUI That Makes Interactive Rebase Feel Like Cheating (heyuan110)](https://www.heyuan110.com/posts/ai/2026-04-10-lazygit-guide/)
- [What makes lazygit better than the integrated Git tools of IDEs/editors? (Discussion #3590)](https://github.com/jesseduffield/lazygit/discussions/3590)
- [The (lazy) Git UI You Didn't Know You Need — bwplotka](https://www.bwplotka.dev/2025/lazygit/)
- [How to Use Lazygit to Improve Your Git Workflow — freeCodeCamp](https://www.freecodecamp.org/news/how-to-use-lazygit-to-improve-your-git-workflow/)
- [Lazygit — JDriven Blog](https://jdriven.com/blog/2026/04/Lazygit)
- [Lazygit: The terminal UI that makes git actually usable — bytesizego](https://www.bytesizego.com/blog/lazygit-the-terminal-ui-that-makes-git-actually-usable)
- [Lazygit Turns 5: Musings on Git, TUIs, and Open Source — Jesse Duffield](https://jesseduffield.com/Lazygit-5-Years-On/)
- [Any opinions on lazygit vs. tig vs. gitui? — Hacker News](https://news.ycombinator.com/item?id=30706467)
- [GitUI vs lazygit — StackShare](https://stackshare.io/stackups/gitui-vs-lazygit)
- [Magit VS lazygit — SaaSHub](https://www.saashub.com/compare-magit-vs-lazygit)
- [I've been using `gitui` after lazygit was getting a bit slow — Hacker News](https://news.ycombinator.com/item?id=39666727)
