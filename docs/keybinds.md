# 使用中キーバインド一覧 (SKK → tmux → zsh)

作成日: 2026-04-29
対象環境: Linux (Manjaro) / MATE / X11 / Ghostty / zsh / tmux 3.6 / fcitx5 + SKK

## 読み方

下位レイヤから上位レイヤへ向けて「**実際に何かのアクションが割り当たっているキー**」を列挙する。
あるレイヤが消費したキーは上位レイヤに届かない。新しいキーを追加するときは、

1. このページの **§1 SKK** で握られていないか
2. **§2 tmux** で衝突しないか
3. **§3 zsh** で衝突しないか

を上から順にチェックする。

レイヤ順 (下が先に評価される):

```
[1] fcitx5 + SKK   ← 最下層、Ctrl+letter を多く消費
[2] tmux           ← prefix = C-b (tmux デフォルト)。配下に独自テーブル
[3] zsh (zle)      ← emacs キーマップ + リポジトリ独自上書き
```

> ⚠️ 表記: `C-x` = Ctrl+x、`M-x` = Alt+x（または Esc x）、`prefix` = `C-b` (tmux デフォルト)。

---

## §1 SKK / Ghostty (下位レイヤ) で使用中のキー

§1.1〜§1.4, §1.6 は **SKK (fcitx5-skk)**、§1.5 だけ **Ghostty** の責務。
両者とも tmux より下位レイヤなので一括して扱う。

`~/.config/fcitx5/profile` の `DefaultIM=skk`、`Rule=default`、`InitialInputMode=Latin`。
libskk / fcitx5-skk 既定キーマップに従う。**モードによって挙動が変わる** ため
モード別に列挙する。

### 1.1 Latin モード (起動時の既定)

「素通し」が基本だが、モード遷移トリガーだけは消費する。

| キー | 動作 |
|---|---|
| `C-J` | **Hiragana モードへ遷移** (kana on) |
| 他のキー | 素通し |

### 1.2 Hiragana / Katakana モード

ローマ字→かな変換中。**ほぼすべての印字可能文字** が消費される。

| キー | 動作 |
|---|---|
| `a-z` `A-Z` | 入力中ローマ字バッファに追加 |
| `Space` | 変換開始 |
| `Enter` / `C-M` / `C-J` | 確定（改行は EggLikeNewLine=True で抑制） |
| `Backspace` / `C-H` | 1 文字削除 |
| `C-G` | 入力 abort |
| `q` | hiragana ⇄ katakana 切替 |
| `Q` | 送り仮名あり変換開始 |
| `l` (小文字 L) | Latin モードへ復帰 |
| `L` (大文字) | 全角 Latin モードへ復帰 |
| `C-Q` | 半角カタカナ切替 |
| `/` | abbrev (英字略語) モード |
| `;` | 英字接頭の準備モード |

### 1.3 変換中モード (Space 後)

| キー | 動作 |
|---|---|
| `Space` | 次候補 |
| `x` | 前候補 |
| `C-N` | 次候補 |
| `C-P` | 前候補 |
| `Enter` / `C-M` / `C-J` | 確定 |
| `C-G` | 変換 abort（確定前に戻る） |
| `>` | 接尾辞変換 |
| `<` | 接頭辞変換 |
| `a` / `s` / `d` / `f` / `j` / `k` / `l` (CandidateChooseKey) | 候補直接選択 |
| `Page_Up` | 候補ページ前 |
| `Next` / `Page_Down` | 候補ページ次 |

### 1.4 libskk default rule が握る Ctrl 系（実装ソース由来）

`/usr/share/libskk/rules/default/keymap/default.json` で以下が `define.keymap` に登録されており、Latin/Hiragana/Katakana/wide-latin/hankaku-katakana 全モードがこれを `include` する。
**SKK が active な間、これらは tmux/zsh に届かない**ため tmux prefix には絶対に使わない。

| キー | 既定動作 (libskk) |
|---|---|
| `C-G` | abort |
| `C-H` | delete (Backspace) |
| `C-I` | complete (Tab) |
| `C-B` | shrink-preedit |
| `C-F` | expand-preedit |
| `C-M` | commit-unhandled (Enter) |

加えて `latin.json` は `C-J` (`set-input-mode-hiragana`)、`hiragana.json` / `katakana.json` は `C-Q` (`set-input-mode-hankaku-katakana`) と `C-J` (`commit`) を追加。

### 1.4.1 user-dir rule は libskk に読まれない (検証結果)

libskk 1.1.1 は `Skk.Rule.find_rules()` で **`/usr/share/libskk/rules/` のみ走査**し、`~/.local/share/libskk/rules/` を見ない (`skk -l` のリストに user dir 配下のルールは出ない)。
過去に `dot_local/share/libskk/rules/tmux-friendly/` を置いて `Rule=tmux-friendly` を試したが `skk.cpp:453 Failed to load rule` でフォールバック動作になり症状が改善しなかった。
回避するにはシステム `/usr/share/libskk/` への配置 (sudo) しかないため、本リポジトリではその方向は取らず **tmux prefix 側を SKK 非干渉キー (`F12`) に振る** 戦略を採用している。

### 1.5 Ghostty レイヤで唯一保持しているキー

Ghostty のデフォルトバインド 70+ 個は `dot_config/ghostty/config` で
`keybind = clear` し、以下 1 つだけ復活させている:

| キー | 動作 | 理由 |
|---|---|---|
| `Ctrl+Shift+V` | paste_from_clipboard (システムクリップボードからの貼付) | tmux の `paste-buffer` ではシステムクリップボードに届かないため、外部アプリ → ターミナルの貼付に必要 |

それ以外（`Ctrl+Tab`, `Ctrl+Shift+T`, `Ctrl+Shift+C`, `Ctrl+,`, `Ctrl+Enter` など）は
すべて素通しになり、tmux / zsh / SKK が処理する。

### 1.6 SKK 非干渉キー (Rule=default のまま使う)

ここに列挙されたキーは、SKK が active でも **上位レイヤに届く**。

> ⚠️ 重要: 過去に「SKK が C-b を握る」と推定して prefix を何度も変更したが、
> **真因は Ghostty + X11 + GTK DE での focus-events 暴発** (#9620) で、
> SKK ではなかった。現在は kitty 主運用に移行したため `set -g focus-events
> on` (resurrect / nvim autoread / pane border 動的更新で必要) でも tmux
> default の `C-b` で問題なく動作する。下表の「不採用」は当時の推定経緯
> メモであり、SKK が実機で C-b 等を喰う再現はない。

| キー | 備考 |
|---|---|
| `C-B` | tmux default。**現在の prefix に採用** (default rule の `shrink-preedit` は Latin モードで実害観測なし) |
| `C-A` | SKK 非消費 (60% keyboard でも home row 左端、screen 系の代替候補) |
| `F1`〜`F12` | SKK 非消費だが 60% keyboard では Fn 同時押し必須 |
| `C-Space` (= `C-@`) | SKK は無視するがターミナル送出層で `^@` が tmux に届かない実機があり不採用 |
| `C-\` | SKK / zsh / tmux いずれも未使用だが、JIS keyboard では `¥` にマップされ tmux に届かないため不採用 |
| `C-]` | SKK / zsh / tmux default いずれも未使用、運用上の都合で見送り |
| `C-D` `C-E` `C-K` `C-L` `C-O` `C-R` `C-S` `C-T` `C-U` `C-V` `C-W` `C-X` `C-Y` `C-Z` | 上位 (tmux/zsh) で利用可能 |
| `C-^` `C-_` | 上位で利用可能 |
| `M-` 系 (Alt) | SKK は通常握らない |

---

## §2 tmux で使用中のキー

prefix は **`C-b`** (tmux デフォルト) (`dot_config/tmux/conf/options.conf`)。
prefix を 1 度押した後、以下のキーが各テーブルで効く。

### 2.1 prefix-table (デフォルト + 本リポジトリ追加分の全リスト)

#### 2.1.1 リポジトリ独自バインド

| キー | 動作 | 定義 |
|---|---|---|
| `prefix + g` | git worktree fzf picker | `bindings.conf` |
| `prefix + s` | session fzf picker（デフォルト `choose-tree -Zs` を上書き） | `bindings.conf` |
| `prefix + C` | claude_table へ遷移 | `bindings.conf` |
| `prefix + h/j/k/l` | pane 移動 (vim 風) | `bindings.conf` |
| `prefix + J/K` | next/previous-window | `bindings.conf` |
| `prefix + L/H` | switch-client n/p | `bindings.conf` |
| `prefix + M-c` | new-window (PWD 継承) | `bindings.conf` |
| `prefix + M-C` | new-session | `bindings.conf` |
| `prefix + |` | split-window -h (PWD 継承) | `bindings.conf` |
| `prefix + _` | split-window -v (PWD 継承) | `bindings.conf` |
| `prefix + q` | kill-pane | `bindings.conf` |

#### 2.1.2 tmux デフォルトバインド (継続して有効)

| キー | 動作 |
|---|---|
| `prefix + Space` | next-layout |
| `prefix + !` | break-pane |
| `prefix + "` | split-window (水平) |
| `prefix + #` | list-buffers |
| `prefix + $` | rename-session |
| `prefix + %` | split-window -h |
| `prefix + &` | kill-window (確認あり) |
| `prefix + '` | select-window by index |
| `prefix + (` / `)` | switch-client p/n |
| `prefix + ,` | rename-window |
| `prefix + -` | delete-buffer |
| `prefix + .` | move-window |
| `prefix + /` | list-keys（特定 keymap 検索） |
| `prefix + 0..9` | select-window by number |
| `prefix + :` | command-prompt |
| `prefix + ;` | last-pane |
| `prefix + <` | display-menu (window) |
| `prefix + =` | choose-buffer |
| `prefix + >` | display-menu (pane) |
| `prefix + ?` | list-keys (全バインド表示) |
| `prefix + D` | choose-client |
| `prefix + E` | select-layout -E |
| `prefix + I` | TPM: install_plugins |
| `prefix + M` | select-pane -M (mark) |
| `prefix + U` | TPM: update_plugins |
| `prefix + Y` | tmux-yank: copy pane PWD |
| `prefix + [` | copy-mode 開始 |
| `prefix + ]` | paste-buffer |
| `prefix + c` | new-window (デフォルト) |
| `prefix + d` | detach-client |
| `prefix + f` | find-window |
| `prefix + i` | display-message |
| `prefix + m` | select-pane -m (mark) |
| `prefix + n` / `p` | next/previous-window |
| `prefix + o` | select-pane (next in window) |
| `prefix + r` | refresh-client |
| `prefix + t` | clock-mode |
| `prefix + w` | choose-tree -Zw |
| `prefix + x` | kill-pane (確認あり) |
| `prefix + y` | tmux-yank: copy-line |
| `prefix + z` | resize-pane -Z (zoom) |
| `prefix + {` / `}` | swap-pane U/D |
| `prefix + ~` | show-messages |
| `prefix + Up/Down/Left/Right` | select-pane (繰り返し可) |
| `prefix + DC` | refresh-client -c |
| `prefix + PPage` | copy-mode 上スクロール |

#### 2.1.3 prefix + Ctrl 系

| キー | 動作 |
|---|---|
| `prefix + C-O` | rotate-window |
| `prefix + C-R` | tmux-resurrect: restore |
| `prefix + C-S` | tmux-resurrect: save |
| `prefix + C-Z` | suspend-client |
| `prefix + C-Up/Down/Left/Right` | resize-pane (1 cell, 繰り返し可) |

#### 2.1.4 prefix + Alt 系

| キー | 動作 |
|---|---|
| `prefix + M-1..M-7` | select-layout (各種) |
| `prefix + M-c` | new-window (PWD 継承、本リポジトリ) |
| `prefix + M-C` | new-session (本リポジトリ) |
| `prefix + M-n` / `M-p` | next/previous-window with alert |
| `prefix + M-o` | rotate-window -D (逆方向) |
| `prefix + M-u` | TPM: clean_plugins |
| `prefix + M-Up/Down/Left/Right` | resize-pane 5 cells |

#### 2.1.5 prefix + Shift 系

| キー | 動作 |
|---|---|
| `prefix + S-Up/Down/Left/Right` | refresh-client offset 10 |

### 2.2 claude_table (`prefix + C` 後の二段目)

| キー | 動作 | 実装 |
|---|---|---|
| `c` | `claude --continue` を現 pane に send-keys | inline |
| `n` | branch fzf → 新規 worktree + 2-pane Claude session | `claude-pick-branch.sh` → `tmux-claude-new.sh` |
| `r` | 現 session 内の claude pane を kill→再起動 | `claude-respawn-pane.sh` |
| `s` | `claude-*` session ピッカー | `claude-pick-session.sh` |
| `k` | session + worktree を一括削除 (確認あり) | `claude-kill-session.sh` |

### 2.3 copy-mode-vi (本リポジトリ追加分)

| キー | 動作 |
|---|---|
| `v` | begin-selection |
| `V` | select-line |
| `C-V` | rectangle-toggle |
| `y` | copy-selection |
| `Y` | copy-line |

### 2.4 root (prefix なしで効く、マウス系のみ)

クリック / ホイール / ドラッグ / ステータスライン操作は全てマウスイベント。
キーボードの `C-x` 系は **root では使われていない**（prefix を経由する必要がある）。

---

## §3 zsh (zle) で使用中のキー

`bindkey -e`（emacs キーマップ）+ リポジトリ独自上書き。
`bindkey -e` 直後に拡張シーケンスや独自定義が乗る。

### 3.1 Ctrl 系（emacs キーマップ標準）

| キー | 動作 |
|---|---|
| `C-@` | set-mark-command |
| `C-A` | beginning-of-line |
| `C-B` | backward-char |
| `C-D` | delete-char-or-list |
| `C-E` | end-of-line |
| `C-F` | forward-char |
| `C-G` | send-break |
| `C-H` | backward-delete-char *(リポジトリで `backward-kill-word` に上書き — §3.4)* |
| `C-I` | expand-or-complete (Tab) |
| `C-J` | accept-line (Enter) |
| `C-K` | kill-line |
| `C-L` | clear-screen |
| `C-M` | accept-line (Enter) |
| `C-N` | down-line-or-history |
| `C-O` | accept-line-and-down-history |
| `C-P` | up-line-or-history *(リポジトリで `^Pd` を pandapdf 関数として bind — §3.4)* |
| `C-Q` | push-line |
| `C-R` | history-incremental-search-backward |
| `C-S` | history-incremental-search-forward |
| `C-T` | transpose-chars |
| `C-U` | kill-whole-line |
| `C-V` | quoted-insert |
| `C-W` | backward-kill-word |
| `C-X` | (二段プレフィックス。下記 §3.2 参照) |
| `C-Y` | yank *(リポジトリで `autosuggest-accept` に上書き — §3.4)* |
| `C-Z` | suspend (デフォルト) *(リポジトリで `fancy-ctrl-z` に上書き — §3.4)* |

### 3.2 `C-X` 二段プレフィックス

| キー | 動作 |
|---|---|
| `C-X C-B` | vi-match-bracket |
| `C-X C-F` | vi-find-next-char |
| `C-X C-J` | vi-join |
| `C-X C-K` | kill-buffer |
| `C-X C-N` | infer-next-history |
| `C-X C-O` | overwrite-mode |
| `C-X C-U` | undo |
| `C-X C-V` | vi-cmd-mode |
| `C-X C-X` | exchange-point-and-mark |
| `C-X *` | expand-word |
| `C-X =` | what-cursor-position |
| `C-X G` / `g` | list-expand |
| `C-X r` | history-incremental-search-backward |
| `C-X s` | history-incremental-search-forward |
| `C-X u` | undo |

### 3.3 Esc / Meta (Alt) 系

| キー | 動作 |
|---|---|
| `M-C-D` | list-choices |
| `M-C-G` | send-break |
| `M-C-H` | backward-kill-word |
| `M-C-I` / `M-C-J` / `M-C-M` | self-insert-unmeta |
| `M-C-L` | clear-screen |
| `M-C-_` | copy-prev-word |
| `M-Space` | expand-history |
| `M-!` | expand-history |
| `M-"` | quote-region |
| `M-$` | spell-word |
| `M-'` | quote-line |
| `M--` | neg-argument |
| `M-.` | insert-last-word |
| `M-0..9` | digit-argument |
| `M-<` / `M->` | beginning/end-of-buffer-or-history |
| `M-?` | which-command |
| `M-A` | accept-and-hold |
| `M-B` | backward-word |
| `M-C` | capitalize-word |
| `M-D` | kill-word |
| `M-F` | forward-word |
| `M-G` | get-line |
| `M-H` | run-help |
| `M-L` | down-case-word |
| `M-N` / `M-P` | history-search-forward/backward |
| `M-Q` | push-line |
| `M-S` | spell-word |
| `M-T` | transpose-words |
| `M-U` | up-case-word |
| `M-W` | copy-region-as-kill |

### 3.4 リポジトリで上書き / 追加しているもの

| キー | 動作 | 定義箇所 |
|---|---|---|
| `C-H` | backward-kill-word（Ctrl+Backspace 用に拡張） | `dot_config/zsh/rc/my_plugins/manjaro.zsh:25` |
| `C-Y` | autosuggest-accept (zsh-autosuggestions の候補確定) | `dot_config/zsh/rc/integrations/zsh-autosuggestions_atload.zsh:9` |
| `C-Z` | fancy-ctrl-z（fg をワンキーで呼び戻す） | `dot_config/zsh/rc/my_plugins/magic_ctrl_z.zsh:11` |
| `^P d` | pandapdf 関数 | `dot_config/zsh/rc/my_plugins/pandapdf.zsh:25` |
| `^[[89;5u` (CSI u 形式 `C-Y`) | autosuggest-accept | `zsh-autosuggestions_atload.zsh:10` |
| `^[[7~` `^[[H` | beginning-of-line (Home) | `manjaro.zsh:3-4` |
| `^[[8~` `^[[F` | end-of-line (End) | `manjaro.zsh:8-9` |
| `^[[2~` | overwrite-mode (Insert) | `manjaro.zsh:13` |
| `^[[3~` | delete-char (Delete) | `manjaro.zsh:14` |
| `^[[5~` `^[[6~` | history-beginning-search-backward/forward (PgUp/PgDn) | `manjaro.zsh:17-18` |
| `^[Oc` `^[Od` `^[[1;5C` `^[[1;5D` | forward/backward-word (Ctrl+Arrow) | `manjaro.zsh:21-24` |
| `^[[Z` | undo (Shift+Tab) | `manjaro.zsh:26` |
| `^G` | navi widget (cheat 検索 → 行に挿入) | `dot_config/zsh/rc/integrations/navi.zsh:14` |

> ⚠️ `^G` は §3.2 で `send-break`、§1 SKK で `abort` でもある。
> SKK Hiragana / 変換中は SKK が先に消費するため zsh まで届かない。
> Latin モード (起動時の既定) でのみ navi widget が発動する。
> tldr (tealdeer) は zle widget を持たず、コマンド名 `tldr` / `tld` / `tldrf` で起動する
> (`Alt+H` は `run-help` 既存使用のため割り当てない)。詳細は
> [`shell_discovery.md`](shell_discovery.md) §4 を参照。

---

## §4 新しいキーを追加するときのチェックリスト

1. **§1 で SKK が握っていないか** — モード横断で安全なのは `C-A` `C-D` `C-E` `C-K`〜`C-Z` (一部除く) / `M-` 系 / 矢印付き修飾。Function キーはレイアウト依存 (60% keyboard で Fn 必須など)
2. **§2 で tmux に空きがあるか** — prefix-table は表示されたものだけが「使用中」、それ以外は空き
3. **§3 で zsh を潰してよいか** — emacs キーマップを潰すと操作感が崩れる。覚悟があるときだけ
4. 採用後はこのページの該当行に追記

## §5 デバッグメモ

### prefix が「2 連打しないと効かない」/「効いたり効かなかったり」する場合

Ghostty + X11 + GTK DE (Cinnamon, MATE) では `set -g focus-events on` が
Ghostty の過剰 focus 通知と相互作用し、tmux が prefix の前段で focus event
を消費する。Wayland/KDE では再現しない。

本リポジトリは現在 kitty を主運用ターミナルとしているため
`dot_config/tmux/conf/options.conf` で `focus-events on` を維持する
(resurrect / nvim autoread / pane border 動的更新が必要)。
Ghostty を併用していて症状が再発する場合は、その端末でセッションを
立てるときだけ `tmux set -g focus-events off` で一時的に無効化すること。

参考: <https://github.com/ghostty-org/ghostty/discussions/9620>
