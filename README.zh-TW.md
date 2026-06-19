# hush

[English](README.md) · **繁體中文**

> 本機、**對 AI agent 安全**的 per-worktree 密鑰管理工具。零服務、零常駐程式。

只要 AI coding agent 進到你的 repo，`.env` 就是個風險：agent 很愛 `cat .env`、`grep -r KEY`，
一個不小心 secret 就跑進模型的 context、或被誤 commit。**hush** 讓 secret 根本不存在你的
worktree 裡——repo 只保留一份「不含值」的宣告，真正的值 age 加密存在 repo 外，而且只在「需要它的那條
命令」的子行程裡才被注入。

```sh
hush run -- npm run dev          # 解密、把 env 只注入這個子行程、然後 exec
```

![hush demo](docs/demo.gif)

- 🔍 在 worktree 裡 `cat .env` / `grep -r KEY` **只找得到 key 名稱，永遠看不到值**。
- 🔐 Secret 以 age 加密存在 repo 外的單一檔案；主金鑰放在 **macOS Keychain**——磁碟上沒有任何明文金鑰檔。
- 🧬 值只注入 `hush run` 的子行程，**絕不**進你的 shell。
- 🤖 **對人絲滑、對 agent 嚴格**。agent／非互動情境（例如 `CLAUDECODE` 有設、沒有 TTY）會被自動偵測並鎖在「能用、不能看」模式。
- 🌳 **天生為 worktree 設計**。宣告檔會 commit，所以新開的 worktree 立刻可用；值依 git branch 區分，並可從 base profile 繼承。

> **平台：僅限 macOS。** hush 依賴 macOS Keychain（主金鑰）與 `hdiutil` RAM disk（讓 `edit` 不會把明文寫到持久化儲存）。

---

## 安裝

**用 Go（推薦——本機編譯，不會被 Gatekeeper 隔離）：**

```sh
go install github.com/allen-hsu/hush@latest
hush install     # 冪等地把  eval "$(hush hook)"  加進 ~/.zshrc
```

**用 Homebrew：**

```sh
brew install allen-hsu/tap/hush
hush install
```

**從原始碼：**

```sh
git clone https://github.com/allen-hsu/hush && cd hush
go build -o ~/bin/hush .   # 確認 ~/bin 在你的 PATH 裡
hush install
```

接著重開 shell（或 `source ~/.zshrc`）。

---

## 快速開始

```sh
cd my-project
hush init                    # 寫出 .hush.toml（會 commit；宣告 key，不含值）
hush import .env --shred     # 把現有 .env 匯入，然後銷毀明文
hush ls                      # 列出宣告的 key + 由哪個 profile 解析（不顯示值）
hush run -- npm run dev      # 解密、把 env 只注入這個子行程、exec
```

整個循環就這樣：宣告 → 匯入 → 執行。你的程式照常讀 `process.env` / `os.environ` /
`vm.envString`——hush 只是負責把值填進它啟動的那個行程的環境。**不需要 SDK、不需要 library、不用改任何程式碼。**

---

## 運作原理

三個部分，職責分得很乾淨：

| | 是什麼 | 放在哪 | 內容 |
|---|---|---|---|
| **宣告** | `.hush.toml` | commit 進 repo | 要哪些 key、怎麼選 profile——**永不含值** |
| **Store** | `store.age` | `~/.config/hush/`（在任何 repo 之外） | 所有值，age 加密，命名空間為 `project → profile → key` |
| **主金鑰** | age 身分金鑰 | macOS Keychain | 首次使用時自動產生；磁碟上沒有明文金鑰檔 |

`hush run` 讀宣告檔、解析當前 **profile**（預設依 git branch）、用 Keychain 金鑰解密 store、
把解出的值注入子行程、然後 `exec`。明文只存在那個子行程的記憶體裡。

### Per-worktree 工作流

`profile = "branch"` 讓值依 git branch 區分，所以每個 worktree／branch 都有自己的一組：

```sh
git worktree add ../feature-x -b feature-x
cd ../feature-x
hush fork                 # 把 base profile 複製成這個 branch 的 profile
hush set DATABASE_URL     # 只覆寫少數幾個不一樣的值
```

commit 進去的 `.hush.toml` 會跟著 checkout 走，所以新 worktree 馬上能用；你只要 `set` 差異的部分。

---

## 指令

| 指令 | 功能 |
|---|---|
| `hush run -- <cmd>` | 解析 profile、把 env 注入子行程、exec。**能用、不能看。** |
| `hush edit` | 在 `$EDITOR` 裡編輯一個 profile（限 TTY；agent 被拒；RAM-disk 支撐）。 |
| `hush set <KEY>` | 設定單一值——互動輸入或 piped stdin。 |
| `hush unset <KEY>` | 從當前 profile 移除一個 key。 |
| `hush ls` | 列出宣告的 key + 由哪個 profile 解析。永不顯示值。 |
| `hush get <KEY>` | 印出某個值（限 TTY；對 agent 拒絕）。 |
| `hush import [path]` | 匯入現有 `.env`。旗標：`--profile`、`--force`、`--shred`。 |
| `hush fork [--from p]` | 把某個 profile 複製到當前 profile。 |
| `hush cp <from> <to>` | 把一個 profile 的值複製到另一個。 |
| `hush init` | 產生一個 `.hush.toml`。 |
| `hush install` | 冪等地把 shell hook 加進 `~/.zshrc`。 |
| `hush hook` | 印出 shell 整合片段（`eval "$(hush hook)"`）。 |
| `hush scrub` | 印出清掉 hush 變數／shim 的 shell 指令，啟動 agent 前用。 |

`--json` 適用於 `ls`、`get`、`set`、`unset`、`import`、`fork`、`cp`，輸出機器可讀格式——給 script 和 agent 用。

---

## Shell 整合

`eval "$(hush hook)"`（`hush install` 會幫你加）在互動 shell 裡提供：

- 你 `cd` 進專案時，一行淡色 banner 顯示當前的 `project · profile`；
- **shim**：對 `.hush.toml` `shims = [...]` 裡的每個命令（你自己挑），打裸命令會自動包起來——
  `npm run dev` 實際以 `hush run -- npm run dev` 執行。值只進那個子行程，你的 shell 保持乾淨。

離開專案時 shim 會被拆掉。當 `CLAUDECODE` / `HUSH_AGENT` 有設（或沒有 TTY）時，**什麼都不裝**——
agent 必須顯式呼叫 `hush run`，永遠不會繼承到 shim 或 shell env。`cd` 的資料來源（`hush context`）
只讀 `.hush.toml`（+ git），不碰 store，所以**永遠不會觸發 Keychain 提示**。

```toml
# .hush.toml — 會 commit、不含值
profile = "branch"          # branch | cwd | fixed:<name>
extends = "base"            # 當前 profile 沒有的 key，往這個 profile 找
keys    = ["DATABASE_URL", "DEPLOYER_KEY"]
shims   = ["npm", "pnpm"]   # opt-in；要自動包 hush run 的命令
```

---

## 安全模型

**hush 能擋住**實務上常見的失誤：

- `cat` / `grep` worktree 檔案而撈出 secret 值；
- 不小心 commit 到值（repo 只會有 key *名稱*）；
- secret 洩進你的持久 shell（進而被你從該 shell 啟動的 agent 繼承）；
- `edit` 把明文寫到持久化儲存（它用 RAM disk；在 APFS/SSD 上 secure-delete 並不可靠，所以「根本不寫」才是唯一可靠的保證）。

**hush 不能擋**同一個使用者底下、*刻意*執行 `hush run -- env` 的程式。任何能解密的本機工具，
共用你 uid 的東西都能呼叫；要堵這個需要獨立的信任域（daemon／per-binary Keychain ACL），
對一個本機單人工具來說刻意不做。目標是擋掉「意外曝光」和「agent 反射性讀檔」——不是擋一個有決心的本機攻擊者。

---

## 散佈 / fork

hush 是單一靜態 Go binary，在 macOS 上有兩個對系統工具的執行期依賴（`security`、`hdiutil`/`diskutil`）。
要出自己的 build，[GoReleaser](https://goreleaser.com) 可以一步產出 Homebrew tap formula 和 GitHub Release。
優先用 `go install` / `brew`（本機編譯）而非直接下載 binary：未簽章、未公證的下載 binary 會被 macOS Gatekeeper 隔離。

---

## 現況

**v0.1 — 已發佈。** `go install github.com/allen-hsu/hush@latest`。

完整指令集已就緒並有測試：
`run · edit · set · unset · ls · get · import · fork · cp · init · install · hook · context · scrub`。
測試涵蓋 store 加解密（round-trip、加密落地、錯金鑰拒絕）、profile/extends 解析、dotenv 解析、RAM-disk 暫存路徑。

刻意不做（針對本機單人工具的設計取捨）：multi-recipient／team 共享、金鑰輪替、非 macOS 平台。

完整設計理念見 [docs/SPEC.md](docs/SPEC.md)。

## 授權

MIT
