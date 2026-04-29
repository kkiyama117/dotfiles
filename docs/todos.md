# Review TODOs — zsh / mise / sheldon

レビュー日: 2026-04-29
対象: `dot_zshenv.tmpl`, `dot_config/zsh/**`, `dot_config/sheldon/plugins.toml`, `dot_config/mise/**`, `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl`, `.executable_password_manager.sh`, `.password_manager.sh`, `.chezmoiignore`

---

## CRITICAL（即時対応）

### C-1. `OPENROUTER_API_KEY` を `~/.zshenv` から外す ✅ 2026-04-29
- 該当: `dot_zshenv.tmpl:143`（削除済み）
- 現状: `export OPENROUTER_API_KEY=...` が `~/.zshenv` に書かれている。zshenv は **すべての zsh 起動**（インタラクティブ・スクリプト・SSH コマンド・コンテナ exec・MCP サーバ等）で読み込まれるため、`/proc/<pid>/environ` 経由でローカル他プロセスから観測でき、子プロセスにも全継承される。
- 対応:
  - [x] `dot_zshenv.tmpl` から API キー設定を削除
  - [x] `dot_config/zsh/rc/private_secrets.zsh.tmpl` に移動（`private_` prefix で 0600）
  - [x] sheldon `[plugins.my_conf_pre_load]` の glob を `{options,secrets}.zsh` に拡張して同期 source
  - [x] `<no value>` ガード（`bw` 未認証時に空 export を防ぐ）をテンプレ側に追加
  - [ ] _option_: API キーを使うコマンド直前にだけ export するラッパー関数 / direnv はフォローアップで検討

### C-2. `bw_session` 関数の自動実行 + `export BW_SESSION` を見直す ✅ 2026-04-29
- 該当: `dot_config/zsh/bin/executable_bw_session`（削除済み） → `dot_config/zsh/rc/functions/bw_session.zsh`（新規）
- 現状（修正前）:
  - `BW_SESSION` は **Bitwarden ボルト全体へのアクセスキー**。`export` すると子プロセスに継承される。
  - L11 で関数を即時呼び出し → source されると毎回 `bw unlock` プロンプトが走る。
  - `apply = ['path']` で PATH 経由実行されると、export が親シェルに届かないという**構造バグ**もある（実質機能していない可能性）。
  - shebang `# /usr/bin/env zsh` が無効（`#!` で始まっていない）。
- 対応:
  - [x] 末尾の自動呼び出し `bw_session` を削除
  - [x] sheldon `[plugins.my_plugins]` の `{functions/*,my_plugins/*}.zsh` glob で source される位置（`rc/functions/bw_session.zsh`）に移動 — defer source される関数定義に切り替え
  - [x] 旧 `executable_bw_session` を `git rm`
  - [x] `bw` 未インストール時のガード追加
  - [x] `bw unlock --raw` の戻り値をチェックし、空文字列なら `export` しない
  - [x] 対のヘルパ `bw_lock` を追加（`unset BW_SESSION`）
  - [ ] _設計判断_: `export BW_SESSION` は **残す**（`chezmoi apply` 等の子プロセスから参照する必要があるため）。代わりに `bw_lock` で明示的に解除する運用にした。「使い終わったら必ず `bw_lock`」を CLAUDE.md/README.md に追記してフォロー予定。

---

## HIGH

### H-1. Bitwarden item UUID をリポジトリから外す ✅ 2026-04-29
- 該当: `dot_zshenv.tmpl:143` (C-1 で削除済み) → `dot_config/zsh/rc/private_secrets.zsh.tmpl` に移動済み → 本タスクで UUID リテラル除去
- 現状: `eaefa7ca-a68b-4de5-bf20-b26000f35f93` が公開リポジトリに直書き。UUID 単独でアクセスは不可だが、漏洩時の標的特定を容易にする情報。
- 対応:
  - [x] `.chezmoi.toml.tmpl`（**ソースルート**）で `promptStringOnce` を使い、`.bitwardenOpenRouterItemId` を chezmoi の `[data]` セクションに保存（保存先 `~/.config/chezmoi/chezmoi.toml` は `.chezmoiignore` で deploy 対象外、リポジトリには混入しない）
  - [x] `private_secrets.zsh.tmpl` をハードコード UUID から `.bitwardenOpenRouterItemId` 参照に変更
  - [x] `bitwardenOpenRouterItemId` が未定義のときは export ブロック全体をスキップ（chezmoi data 不在時に空 export を防ぐ）
  - [x] **fix (2026-04-29)**: 当初 `dot_config/chezmoi/chezmoi.toml.tmpl` に置いたが、これは deploy ターゲットとして処理されるため `promptStringOnce` が `function not defined` エラーになる。`.chezmoi.toml.tmpl`（ソースルート、`chezmoi init` 時専用）に移動して解消（commit `7399c71`）
- **移行手順** (適用前に必須):
  1. `chezmoi init kkiyama117 --apply=false` を実行 → UUID 入力プロンプト → `eaefa7ca-a68b-4de5-bf20-b26000f35f93` を入力
  2. または手動で `~/.config/chezmoi/chezmoi.toml` を編集し、`[data]` セクションに `bitwardenOpenRouterItemId = "eaefa7ca-a68b-4de5-bf20-b26000f35f93"` を追加
  3. `BW_SESSION=$(bw unlock --raw); chezmoi diff` で `secrets.zsh` の展開結果に値が入ることを確認

### H-2. `eval $(keychain ... $GPG_KEY_ID)` の引用符抜け + 未定義変数 ✅ 2026-04-29
- 該当: `dot_config/zsh/dot_zshrc:18`
- 現状: `$GPG_KEY_ID` がリポジトリ内に定義されておらず空展開。`eval` が裸の `$(...)` でメタキャラ脆弱性。
- 対応:
  - [x] `eval "$(keychain -q --eval --agents ssh,gpg "$_gpg_key_id")"` に修正（引用符付き）
  - [x] `$GPG_KEY_ID` の代替として `git config --global user.signingkey` から実行時取得。これは `dot_config/git/config.tmpl:12` で Bitwarden から既に注入済みのキー ID なので、新規 bw call 不要
  - [x] `(( $+commands[keychain] ))` ガード、`signingkey` が空のときは `ssh` agent のみで keychain 起動するフォールバック追加

### H-3. sheldon プラグインに pin を入れる（サプライチェーン） ✅ 2026-04-29
- 該当: `dot_config/sheldon/plugins.toml` 全 GitHub プラグイン
- 現状: `tag` / `rev` 指定なし → リポジトリ乗っ取り or 悪意ある merge があれば次回 `sheldon source` で実行される。
- 対応:
  - [x] `zsh-defer` → `rev = "53a26e287fbbe2dcebb3aa1801546c6de32416fa"` (リリースタグ無しのため master HEAD で固定)
  - [x] `zsh-autosuggestions` → `tag = "v0.7.1"`
  - [x] `zsh-completions` → `tag = "0.36.0"`
  - [x] `fast-syntax-highlighting` (auscompgeek フォーク) → `tag = "v1.55"`
  - [x] `tmux-xpanes` → `tag = "v4.2.0"`
  - [ ] _follow-up_: `auscompgeek/fast-syntax-highlighting` のメンテ状況を再確認し、本家 `zdharma-continuum` 系への乗り換え検討

### H-4. mise のセキュリティ設定を引き締める ✅ 2026-04-29
- 該当: `dot_config/mise/config.toml:26,28,30`
- 現状: `paranoid = false`、`yes = true`、`not_found_auto_install = true` の組み合わせはタイポスクワッティング・チェックサム省略のリスクを増幅。
- 対応:
  - [x] `paranoid = true`
  - [x] `yes = false`（プロンプト復活）
  - [x] `not_found_auto_install = false`
  - [ ] `trusted_config_paths = ['~/programs']` の範囲をプロジェクト単位に縮小（TODO コメントのみ残置、運用への影響を避けるため別 task で）

### H-5. `bw_session` の構造バグ（C-2 と同根） ✅ 2026-04-29
- 該当: `dot_config/sheldon/plugins.toml:85-87`, `dot_config/zsh/bin/executable_bw_session`（削除済み）
- 現状: `apply = ['path']` で PATH 追加されるが、外部実行された子プロセス内で `export` しても親に効かない。
- 対応: C-2 とまとめて修正済（`rc/functions/bw_session.zsh` で defer source される関数定義に切り替え）

### H-6. `for_development.zsh` の壊れた path 展開 ✅ 2026-04-29
- 該当: `dot_config/zsh/dot_zshenv/for_development.zsh:33`（旧、削除済み） → `dot_config/zsh/rc/for_development.zsh`
- 現状: `path=($home/programs/q-e/bin(N-/) $path)` の `$home`（小文字）が誤り。
- 対応:
  - [x] `$HOME` に修正

### H-7. `for_development.zsh` の二重 export / 相対パス ✅ 2026-04-29
- 該当: `dot_config/zsh/dot_zshenv/for_development.zsh:8-11, 14-16, 29-32, 36-37, 40-41, 44-45`（旧、削除済み）
- 現状:
  - `GMS_PATH`, `BOOST_ROOT`, `LD_LIBRARY_PATH`, `LDFLAGS`, `CPPFLAGS` が 2 回ずつ定義
  - `ANDROID_SDK_ROOT` を `/opt/android` → `/opt/android-sdk` で連続上書き
  - `ANDROID_JAVA_HOME=opt/android-studio/jre` が先頭 `/` 抜け（相対パス）
  - `LD_LIBRARY_PATH=/usr/local/lib/pkgconfig` は本来 `PKG_CONFIG_PATH`
- 対応:
  - [x] 重複 export を 1 回ずつに整理（GMS_PATH / BOOST_ROOT / cuda path 等）
  - [x] `ANDROID_JAVA_HOME` を `/opt/android-studio/jre` に修正、`ANDROID_SDK_ROOT` 連続上書きも解消
  - [x] `pkgconfig` パスを `PKG_CONFIG_PATH` 側へ移動、`LD_LIBRARY_PATH` は `/usr/local/lib:/usr/lib` のみに整理
  - [x] `LDFLAGS` / `CPPFLAGS` は単一 export で連結に変更（途中 prepend を 1 行化）

### H-8. `WORDCHARS` のパターン置換バグ ✅ 2026-04-29
- 該当: `dot_zshenv.tmpl:104`
- 現状: `WORDCHARS=${WORDCHARS//\/[&.;]}` は「`/` の直後に `[&.;]` のいずれか」を削る挙動になり意図と異なる。
- 対応:
  - [x] `WORDCHARS=${WORDCHARS//[\/&.;]/}` に修正

### H-9. `[[ -v "$DEBUG" ]]` のロジックバグ ✅ 2026-04-29
- 該当: `dot_zshenv.tmpl:4`, `dot_config/zsh/dot_zshrc:55`
- 現状: `-v` の引数は変数名であって値ではない。`$DEBUG` を渡すと展開後の値（多くは空）を変数名として判定。
- 対応:
  - [x] `[[ -v DEBUG ]]` に変更（両ファイル）

### H-10. `completion.zsh` で未定義関数 `dir_color` を呼んでいる ✅ 2026-04-29
- 該当: `dot_config/zsh/rc/completion.zsh:4`
- 現状: `dir_color` という関数はリポジトリ内に存在しない。`aliases.zsh:7` の `alias dircolor=...` （アンダースコアなし）と混同？
- 対応:
  - [x] `eval "$(dircolors -b "$XDG_CONFIG_HOME/dircolors")"` を inline 化（`commands[dircolors]` と `[[ -r ... ]]` でガード）
  - [ ] _follow-up_: `aliases.zsh:7` の `dircolor` alias は手動再読込用として残置。整理は別 task で

### H-11. `mkdir $XDG_DATA_HOME/zsh` に `-p` がない ✅ 2026-04-29
- 該当: `dot_zshenv.tmpl:82-85`
- 現状: ディレクトリ存在時にエラー。
- 対応:
  - [x] `mkdir -p $XDG_DATA_HOME/zsh` に変更

### H-12. `zsh-syntax-highlighting` と `fast-syntax-highlighting` の重複ロード ✅ 2026-04-29
- 該当: `dot_config/sheldon/plugins.toml:51-57`
- 現状: 両方 defer で読み込んでおり、最後勝ちで片方は無駄に起動コストを支払う。
- 対応:
  - [x] `[plugins.zsh-syntax-highlighting]` (zsh-users) を削除し、`fast-syntax-highlighting` (auscompgeek フォーク) のみに集約
  - [ ] _follow-up_: H-3 で auscompgeek フォークのメンテ状況と pin 運用を確認後、本家 `zdharma-continuum/fast-syntax-highlighting` への乗り換えを検討

---

## MEDIUM

### M-1. mise `shorthands.toml` のリンク切れ ❌ 誤判断 (2026-04-29)
- 該当: `dot_config/mise/config.toml:32`, `dot_config/mise/empty_shorthands.toml`
- 当初の判断: ファイル名不一致と思い `git mv empty_shorthands.toml shorthands.toml` でリネーム → mise が `~/.config/mise/shorthands.toml: No such file or directory` を WARN
- 真因: chezmoi では `empty_` は「空ファイルでもデプロイする」ことを示す**メタデータ prefix**。target 名は **`empty_` を strip した `shorthands.toml`** で正しい。prefix を落とすと chezmoi は 0-byte ファイルを skip して target が消える。
- 対応:
  - [x] リネームを差し戻し（`empty_shorthands.toml` に復元）
  - [x] `chezmoi apply ~/.config/mise/shorthands.toml` で 0-byte target 再生成を確認
- 学び: `empty_` / `private_` / `executable_` / `readonly_` 等は target 名から strip される chezmoi prefix。prefix が無いと挙動が変わるので削除しない。

### M-2. `mise` の `env_file = '.env'` で `.env` 自動読込 ✅ 2026-04-29
- 該当: `dot_config/mise/config.toml:36`
- 現状: カレントの `.env` を自動展開。サードパーティ repo に `cd` するだけで環境汚染の可能性。
- 対応:
  - [x] `env_file = '.env'` をコメントアウト（明示が必要なら direnv 等で運用）

### M-3. `gpg-connect-agent` を未インストール環境でも無条件実行 ✅ 2026-04-29
- 該当: `dot_config/zsh/dot_zshrc:22, 27`
- 対応:
  - [x] 両箇所に `(( $+commands[gpg-connect-agent] )) && \` ガードを追加

### M-4. `~/.local/bin/env` を無条件 source ✅ 2026-04-29
- 該当: `dot_config/zsh/dot_zshrc:60`
- 現状: chezmoi 管理外のスクリプトを source している。`INSTALLER_NO_MODIFY_PATH=1` 設定済みで通常生成されないはずなので冗長＋潜在リスク。
- 対応:
  - [x] 行を削除（理由コメントは残置）

### M-5. `cargo/env` の冗長 source ✅ 2026-04-29
- 該当: `dot_config/zsh/dot_zshrc:59`
- 現状: `$CARGO_HOME/bin` は `dot_zshenv.tmpl:116` で既に PATH に追加済み。
- 対応:
  - [x] 行を削除（理由コメントは残置）

### M-6. `dot_zprofile` の二重定義 / ハードコード ✅ 2026-04-29
- 該当: `dot_config/zsh/dot_zprofile:9-12`
- 現状:
  - `if [ -n "$ZSH_VERSION" ]` ブロック**外**で env を export しており bash でも適用される
  - `GTK2_RC_FILES` を `dot_zshenv.tmpl:149` と二重定義（zprofile が後勝ち）
  - `BROWSER=/usr/bin/google-chrome-stable` ハードコード
- 対応:
  - [x] `GTK2_RC_FILES` の定義場所を `dot_zshenv.tmpl` に一元化（zprofile 側を削除）
  - [x] `BROWSER` を `for ... commands[$_browser]` で動的解決（`google-chrome-stable / google-chrome / chromium / firefox` の順）
  - [x] `dot_zprofile` は ZDOTDIR 配下にあり zsh login shell 専用のため `ZSH_VERSION` 判定ブロックを削除（実質無意味だった）

### M-7. `cd` 関数の二重再定義 ✅ 2026-04-29
- 該当: `dot_config/zsh/rc/integrations/zoxide.zsh:95`, `dot_config/zsh/rc/integrations/onefetch.zsh:2`
- 現状: defer ロード順で後勝ち。最終的に `onefetch.zsh` の `cd` が有効化され、内部で `z "$@"` を呼ぶ。
- 対応:
  - [x] `onefetch.zsh` を `chpwd_functions` フックに切り替え（`__onefetch_chpwd`）、`cd` の上書きを廃止
  - [x] `LAST_REPO`（グローバル）→ `__ONEFETCH_LAST_REPO`（typeset -g, 衝突低減）へリネーム
  - [x] `git rev-parse --is-inside-work-tree` で worktree チェックを早期 return に整理

### M-8. `.chezmoiignore` のサンプル残骸 ✅ 2026-04-29
- 該当: `.chezmoiignore:8-15`
- 対応:
  - [x] `*.txt`, `*/*.txt`, `backups/`, `backups/**` のサンプルを削除し、重複していた `/docs/**/*` を `docs/` に統合

### M-9. `dot_zshenv` がディレクトリ扱い ✅ 2026-04-29
- 該当: `dot_config/zsh/dot_zshenv/for_development.zsh`
- 現状: chezmoi が `dot_zshenv` をディレクトリと認識しているため、target は `~/.config/zsh/.zshenv/for_development.zsh` という配置になり、zsh は **読み込まない**。
- 対応:
  - [x] `for_development.zsh` を `dot_config/zsh/rc/` 配下に移動、sheldon `[plugins.my_conf_pre_load]` の glob を `{options,secrets,for_development}.zsh` に拡張（同期 source）
  - [x] 旧 `dot_zshenv/` ディレクトリは `git rm` で除去（空になり自動削除）
  - **移行注意**: `chezmoi apply` 時に target 側 `~/.config/zsh/.zshenv/for_development.zsh` も自動削除されるはず。残った場合は手動で `rm -r ~/.config/zsh/.zshenv` する

---

## LOW

### L-1. `aliases.zsh` の危険 alias `gu` ✅ 2026-04-29
- 該当: `dot_config/zsh/rc/aliases.zsh:61`
- 現状: `alias gu='git add . && git commit && git push'` — `add .` で意図しないファイルを巻き込む。
- 対応:
  - [x] alias を削除（`git add -p` を直接打つ運用に変更）

### L-2. `aliases.zsh` の `$HOME` ハードコード ✅ 2026-04-29
- 該当: `dot_config/zsh/rc/aliases.zsh:65, 68`
- 現状:
  - `alias claude="$HOME/.local/bin/claude"` — PATH に既にあるため冗長
  - `[ -s "/home/kiyama/.bun/_bun" ] && source "/home/kiyama/.bun/_bun"` — `/home/kiyama` が直書き
- 対応:
  - [x] `claude` alias を削除
  - [x] bun の path を `$HOME/.bun/_bun` に変更し、`[[ -r ... ]]` ガードに変更

### L-3. `navi.zsh` のスペースバグ ✅ 2026-04-29
- 該当: `dot_config/zsh/rc/integrations/navi.zsh:1`
- 現状: `alias navit= navi --tldr` （`=` の後にスペース）
- 対応:
  - [x] `alias navit='navi --tldr'` に修正

### L-4. `pandapdf.zsh` に個人特定情報がハードコード ✅ 2026-04-29
- 該当: `dot_config/zsh/rc/my_plugins/pandapdf.zsh:7, 15`
- 現状: `6530365061_木山.pdf` という学籍番号風数字＋姓が公開リポジトリに含まれる。
- 対応:
  - [x] 出力ファイル名を環境変数 `PANDAPDF_OUTPUT`（既定 `output.pdf`）で上書き可能に変更
  - [x] 直接呼び出し時は `pandapdf <page> [output.pdf]` で第 2 引数指定可能
  - [x] PII（学籍番号風 ID + 姓）をリポジトリから除去

### L-5. rustup の curl-pipe-sh ✅ 2026-04-29
- 該当: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl:36`
- 現状: `--proto '=https' --tlsv1.2` 指定済みで実害は小さいが、curl-pipe-sh アンチパターン。
- 対応:
  - [x] `sh -s -- --no-modify-path -y` を追加（`INSTALLER_NO_MODIFY_PATH=1` の二重防御 + run_once での非対話実行を明示）

### L-6. `HISTSIZE=200` と `SAVEHIST=100000` の乖離 ✅ 2026-04-29
- 該当: `dot_zshenv.tmpl:88-89`
- 現状: インメモリ 200・ディスク 10 万件。`hist_ignore_space` 有効でシークレット引数の漏洩は緩和されている。
- 対応:
  - [x] `HISTSIZE=10000` に変更（SAVEHIST=100000 と整合）

### L-7. integration alias の重複 ✅ 2026-04-29
- 該当: `aliases.zsh` と `integrations/{lsd,bat,fd,ripgrep}.zsh`
- 現状: `ls` / `cat` / `find` / `grep` を 2 ファイルで上書き。意図的だが見通しが悪い。
- 対応:
  - [x] `aliases.zsh` の ls 群に「coreutils フォールバック・lsd/bat/fd/rg があれば integrations 側で上書きされる」旨をコメントで明記

### L-8. `.password_manager.sh` と `.executable_password_manager.sh` の二重管理 ✅ 2026-04-29
- 該当: `.executable_password_manager.sh`, `dot_config/chezmoi/.password_manager.sh`, `dot_local/share/chezmoi/dot_password_manager.sh`
- 現状: アクティブなのは source root の `.executable_password_manager.sh` のみ。他 2 箇所は古いコピー（`bw` チェックも非推奨形式）が放置されていた。
- 対応:
  - [x] `git rm dot_config/chezmoi/.password_manager.sh dot_local/share/chezmoi/dot_password_manager.sh`
  - [ ] _follow-up_: `dot_local/share/chezmoi/dot_keep`（chezmoi ソース dir 内への自己参照）も整理候補だが、本タスクの範囲外として残置

### L-9. `dot_config/zsh/completions/` と `dot_zfunc/` の二重補完 ✅ 2026-04-29
- 該当: `dot_config/zsh/completions/`, `dot_config/zsh/dot_zfunc/`
- 現状: 同じ補完ファイル群（`_btm`, `_cargo`, `_chezmoi`, ...）が両方に存在。`plugins.toml` の `my_fpaths` は `~/.config/zsh/.zfunc` のみ参照。
- 対応:
  - [x] `dot_config/zsh/completions/` を `git rm -r`（fpath 未参照のため dead code）
  - [x] `dot_config/zsh/dot_zshrc` のコメントアウト済み旧 fpath 行を、廃止の経緯を残す注記に置換
  - 注: 両ディレクトリで内容が異なる補完（`_cargo`, `_deno`, `_pueue`, `_rustup`, `_rye`, `_volta`）は `dot_zfunc/` 側を生かしている

---

## Follow-ups（既存タスクから派生したもの）

### F-1. `upd` / `topgrade` 実行時に `chezmoi apply` も自動で走らせる
- 背景: C-2 / 関連で `topgrade.toml` の `disable` に `chezmoi` を追加し、`upd` alias からも除外した。これは bw 解錠を伴う chezmoi step が unattended 実行で失敗するのを防ぐためだが、結果として **定期更新フローから chezmoi apply が抜け落ちている**。本来は `upd` 一発で chezmoi も最新になってほしい。
- 候補:
  - **A**: `upd` を function 化し、`topgrade ... && chezmoi_apply` で連結する（一番素直）
  - **B**: `topgrade.toml` の `[post_commands]` で `chezmoi_apply` を呼ぶ（topgrade の通知系・タイミング制御に乗る）
  - **C**: topgrade の `[commands]` カスタムコマンドとして `"Chezmoi (with bw)" = "chezmoi_apply"` を登録する
- 注意点:
  - `chezmoi_apply` は内部で `bw unlock` のパスワード入力を要求するため、unattended（cron 等）で `upd` を流す場合は別途 `BW_SESSION` を事前 export する運用にする
  - `bw_session` / `chezmoi_apply` は zsh 関数なので、topgrade 経由で呼ぶ場合は `zsh -ic 'chezmoi_apply'` のように interactive shell 経由になる（PATH に置く実体ではないため）
- 対応:
  - [ ] A / B / C のいずれを採用するか決める（A 推奨）
  - [ ] 採用案を実装
  - [ ] CLAUDE.md / README.md に「`upd` で chezmoi も適用される」旨を追記

### F-2. `bw_lock` 運用の文書化 ✅ 2026-04-29
- 背景: C-2 で `BW_SESSION` の export は残しつつ、対の `bw_lock` で明示的に解除する運用とした。利用者（自分）が忘れると long-lived な bw セッションが env に残る。
- 対応:
  - [x] `CLAUDE.md` の「Bitwarden セッション」節を `bw_session` / `bw_lock` ヘルパ前提に書き換え + 注意書き追加
  - [x] `README.md` の "update" 節を `bw_session` → `chezmoi apply` → `bw_lock` の流れに更新
  - [ ] _follow-up_: `precmd` フックで一定時間アイドルなら自動 `bw_lock` する案は別タスクで検討（過剰になりやすいため、現時点では明示運用に留める）

---

## sheldon を脇に置く判断材料

現状 sheldon が担っている責務:
- 外部 zsh プラグインの取得・defer ロード
- `mise activate` / `starship init` / `compinit` / `colors` の inline 実行
- ローカルファイル（`rc/{options,aliases,completion,functions/*,my_plugins/*}.zsh`, `integrations/*.zsh`）の取り込み
- PATH 追加（`~/.config/zsh/bin`）
- fpath 追加（`~/.config/zsh/.zfunc`）

移行候補:
- **A. zinit / antidote へ移行**: lockfile + pin 標準化（H-3 が同時解決）。投資対効果が最も高い。
- **B. sheldon を残し pin 運用**: `[plugins.*]` に `rev` か `tag` を必須化するだけ。最小コスト。
- **C. 完全撤廃**: `.zshrc` 直書き + git submodule。依存ゼロだが defer / キャッシュ自前。

候補 B → A の段階移行が現実的。

---

## 対応順の推奨

1. ✅ C-1 / C-2（API キーと BW_SESSION） — セキュリティ最優先
2. ✅ H-6 / H-7 / H-8 / H-9 / H-10 / H-11 — 動作バグ
3. ✅ H-3 / ✅ H-4 / ✅ H-1 / ✅ H-2 — サプライチェーン・シークレット周り
4. ✅ H-5 / ✅ H-12 / ❌ M-1 (誤判断・差し戻し) / ✅ M-9 — 不要・壊れた構成
5. ✅ M-2 / ✅ M-3 / ✅ M-4 / ✅ M-5 / ✅ M-6 / ✅ M-7 / ✅ M-8、✅ L-1 / ✅ L-2 / ✅ L-3 / ✅ L-4 / ✅ L-5 / ✅ L-6 / ✅ L-7 / ✅ L-8 / ✅ L-9 — クリーンアップ
6. F-1（要 A/B/C 判断） / ✅ F-2 — 派生フォローアップ

### 残タスクサマリー
- **HIGH**: 全完了
- **MEDIUM**: 全完了（M-1 は誤判断・差し戻し済）
- **LOW**: 全完了
- **Follow-ups**:
  - F-1 (`upd` で chezmoi apply 統合) — A/B/C のどれを採用するか要決定（A 推奨）
  - ✅ F-2 (`bw_lock` 文書化)
