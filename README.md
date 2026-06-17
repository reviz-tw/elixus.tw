# Elixus

關於數位身分、資料自主、雲端封建、社群網路與新聞媒體的靜態網誌。

以 [Hugo](https://gohugo.io/) 產生靜態網站，部署到 GitHub Pages（自訂網域 `elixus.tw`）。
寫作透過一個**只在本機執行**的瀏覽器 markdown 編輯器，該編輯器不會被部署，因此正式站台沒有任何後台可被濫用。

## 環境需求

- Hugo extended ≥ 0.162
- Go ≥ 1.26（本地編輯器用）

## 日常寫作流程

```bash
make dev      # 同時啟動 Hugo 預覽 + 本地編輯器
```

- Hugo 預覽：<http://localhost:1313>（含草稿）
- 編輯器：<http://localhost:1314>（僅綁 127.0.0.1）

編輯器是**所見即所得（WYSIWYG）**的，底層仍存成標準 Markdown：

1. 「＋ 文章」開新文章；「＋ 頁面」開新獨立頁面（如「關於」）。
2. 填標題、slug（英文檔名）等；文章另有標籤與草稿。內文直接打字、用工具列套用格式（也可切到右下角的 Markdown 模式手寫）。
3. **圖片**：直接貼上（⌘V）或拖曳進編輯器，會自動存到 `static/images/`（內容雜湊命名）並插入 `![](/images/…)`。
4. ⌘S（或按「儲存」）：文章寫入 `content/posts/<slug>.md`，頁面寫入 `content/<slug>.md`。
5. 切回 <http://localhost:1313> 看 Hugo 實際渲染結果。

左側清單會分「頁面」與「文章」兩區；點任一項即可載入編輯。

> 預設新文章是 **草稿（draft）**，不會被正式 build 出去。確定要發布時，把草稿取消勾選再儲存。
> 頁面預設即為發布。

**選單與編輯器的關係**：「筆記」（首頁）與「主題」（tags）是 Hugo 依文章內容**自動產生**的，沒有對應檔案、也不需手動編輯；只要增刪文章或調整其 tags，它們就會更新。真正能在編輯器裡改的獨立頁面是「關於」這類 `content/*.md`。

### 其他指令

```bash
make serve            # 只開 Hugo 預覽
make edit             # 只開本地編輯器
make new SLUG=my-post # 用 archetype 建空白草稿
make build            # 產生正式站台到 public/（不含草稿）
make clean            # 清除產生檔
```

## 部署

push 到 `main` 後，`.github/workflows/deploy.yml` 會自動以 Hugo build 並發布到 GitHub Pages。
**不需要也不應該** commit `public/`。

首次設定需在 GitHub repo → Settings → Pages 把 **Source 設為 GitHub Actions**，
並設定自訂網域 `elixus.tw`（`static/CNAME` 已備妥）。

## 為什麼編輯器是獨立的本地工具？

核心設計約束：寫作介面只該存在於作者的機器上。
編輯器是 `tools/editor/` 裡一支獨立的 Go 程式，只綁 `127.0.0.1`，且**不在 Hugo 的 build 流程內**，
所以它永遠不會出現在 `public/`，也就不會被部署。正式站台是純靜態 HTML，沒有任何可寫入的端點。

## 設計

刻意低調、暗色、留白多。基於資料自主的主題，**不使用 Google Fonts 或任何第三方追蹤腳本**，
全用系統字體。

## 編輯器的第三方元件

WYSIWYG 編輯體驗用 [Toast UI Editor](https://ui.toast.com/tui-editor)（MIT 授權），
bundle 已**下載 vendor 進 `tools/editor/vendor/`**（非 CDN，可離線、無執行期第三方請求），
並關閉它預設會送出的 `usageStatistics` 主機名統計。這些只存在於本機編輯器，**不會進入發布的靜態站台**。

> 圖片以 `/images/…` 絕對路徑寫入 markdown，對應正式網域 `https://elixus.tw/` 的根路徑。
