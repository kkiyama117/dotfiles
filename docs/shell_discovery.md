# シェルコマンド発見系の役割分離ポリシー

作成日: 2026-04-30
対象: zsh autocomplete / tldr (tealdeer) / navi の3つを混在運用しているケース
前提環境: Linux (Manjaro) / zsh + sheldon / tmux

このドキュメントは「どれを使うか」ではなく「いつ・なぜ・どれが起動するか」を決めるためのポリシー。
重複ストック(同じ知識が3か所に散る)と、新規マシンでの再現性低下を防ぐのが目的。

関連: [`todos.md`](todos.md) の `S-1` (本ポリシーのフェーズ A 該当)、[`keybinds.md`](keybinds.md) (キーバインド全体の衝突管理)。

---

## 1. 3層の責務分離

| レイヤ | ツール | 用途 | 起動タイミング | 入力 → 出力 |
|--------|--------|------|----------------|-------------|
| L1 | **zsh autocomplete** (compinit + fzf-tab) | 既に打ち始めたコマンド/引数の補完 | Tab(無意識) | 既入力 → 候補挿入 |
| L2 | **tldr (tealdeer)** | コマンド単位の**公式**用例参照 | `tldr <cmd>` 明示 | コマンド名 → 短いチートシート表示 |
| L3 | **navi** | **自分用 / プロジェクト固有**のワンライナーをパラメタライズ実行 | キーバインド or `navi` | 検索 → 引数フォーム → 実行 or 挿入 |

各レイヤは互いに**用途が排他**であり、上位レイヤが下位レイヤを置き換えてはいけない。
特に L3 (navi) に L2 相当 (tldr で済む公式コマンドの基本用法) を貯め始めると重複ストックが発生する。

## 2. 「どれを使うか」判断フロー

```
打鍵中 ──────────────────────────────────► Tab (L1: zsh)
   │
   └── 打ち始められない / コマンド名が不明
        │
        ├── 一般的な公開コマンドの基本用法を知りたい ──► tldr <cmd> (L2)
        │
        └── 自分の作業特有のコマンド列を呼び出したい
             │
             ├── 引数を毎回変えて実行する ─────────────► navi (L3)
             ├── 完全に固定の短い1〜2語 ────────────────► alias (rc/aliases.zsh)
             └── 複数行スクリプト / 副作用あり ────────► dot_local/bin/ にスクリプト化
```

## 3. 禁則 (運用ルール)

3-1. **navi に tldr 相当を貯めない**
   - 例: `git log --oneline` のような公式の基本用法は tldr で十分
   - navi に書くのは「**自分の dotfiles / 環境 / 業務文脈に依存する**」コマンドのみ
   - 判定基準: 他人のマシンでそのまま動くなら tldr 圏、`$BW_SESSION` や `chezmoi cd` 前提なら navi 圏

3-2. **alias を navi に全移管しない**
   - 1〜2 単語で頻打する alias (`g=git`, `ll=ls -al` 等) はキータイプ速度の利得があるため残す
   - 移行候補: 引数を3つ以上取る or 30 文字以上の alias / 関数

3-3. **キャッシュ/cheat の置き場は chezmoi 管理下に固定**
   - tealdeer config: `dot_config/tealdeer/config.toml` (フェーズ B で導入)
   - navi cheats: `dot_config/navi/` 配下 (フェーズ B で導入)
   - 機密を含む cheat は `private_*` プレフィックスで 0600 配置

3-4. **キーバインドは [`keybinds.md`](keybinds.md) で一元管理**
   - 新規バインド前に SKK → tmux → zsh の順で衝突確認

## 4. キーバインド (フェーズ B 実機確認結果)

| 動作 | 採用 | 備考 |
|------|------|------|
| zsh 補完候補 | `Tab` (既定) | fzf-tab と一体運用 |
| tldr 起動 | コマンド名 (`tldr` / `tld` / `tldrf`) | `Alt+H` は zsh `run-help` で使用中のため割り当てなし。`tldrf` は skim/fzf 連携 |
| navi widget | `Ctrl+G` (navi デフォルト) | `^G` は §1 SKK の `abort` と zsh `send-break` と衝突するが、**SKK Latin モード時のみ** zsh まで届くため実用上は同居可。Hiragana / 変換中は SKK が先に消費 |
| navi 起動 (コマンド) | `nv` / `navi` / `navit` (`navi --tldr`) | キーバインドが SKK で消費されたときの fallback |

> フェーズ A 時点では「`Alt+H` / `Ctrl+G`」を案として出していたが、実機の
> [`keybinds.md`](keybinds.md) §3.3 / §3.2 を当たった結果:
> - `Alt+H` = `run-help` (既存) → tldr に割り当てない
> - `Ctrl+G` = `send-break` (zsh) / `abort` (SKK) → navi widget でこれを上書き
>   する形で採用 (上記の通り SKK 飲み込み制約あり)
>
> 詳細表は [`keybinds.md`](keybinds.md) §3.4 末尾を参照。

## 5. 段階導入 (todos.md `S-1` と同期)

- **フェーズ A** (本ドキュメント): 役割分離を明文化
- **フェーズ B**: chezmoi に `dot_config/tealdeer/` と `dot_config/navi/` を取り込み、`PACKAGES` 追加、widget / キーバインドを `rc/functions/` に整理
- **フェーズ C**: 既存 `dot_config/zsh/rc/aliases.zsh` を棚卸しし、上記 §3-2 の基準で `*.cheat` 化

各フェーズの具体的なチェックリストは [`todos.md`](todos.md) `S-1` を参照。
