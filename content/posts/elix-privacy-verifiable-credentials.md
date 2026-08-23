+++
title = "Elix （一）：打造一個保護隱私，又避免被濫用的安全公共討論空間"
author = "hcchien"
date = 2026-08-23
draft = false
tags = ["Elix", "隱私", "DID", "數位身分", "Verifiable Credentials"]
+++

![](/images/img-3dc3f777c3a1.jpg)

最近 Elix 上了 1.0.5 iOS 版，Android 目前還在送審的路上

也開始有朋友傳訊息表達對於這個設計的興趣，畢竟目前各種社群媒體已經被各種沒有朋友的帳號、或是資訊操弄佔領。甚至我前幾天發的幾篇文，包含轉發紐約時報報導台灣民防的文章，都有類似的帳號來留言。就更不用說一些比較敏感的社團，常常會有奇怪的帳號入侵。

很多人覺得這些問題真的很困擾，但想像上要解決這些問題，似乎又要犧牲大家的隱私，好像要有一個可以正常、而且不需實名制的公共討論空間已經難以達成。

而 Elix 的眾多目的之一就是希望在這個前提底下，先有一個安全的網路公共空間，接著我們再來討論怎麼識別假訊息，或是因為演算法造成的社會極化等等困境。當然這些後續的問題也是 Elix 希望可以跟各位使用者共同面對的問題之一。

我們希望實作能保有匿名，而又安全公共討論空間的方式，還是需要透過可受公眾信任的方式來達成，現在 App 採用的是護照以及電子自然人憑證。對絕大多數的使用者來說，這些都是極度敏感的個人資料，因此要可以安心使用 Elix App 應該還是必須先確認自己進行身分驗證的方式是受到保障的。

目前 Elix 的完整原始碼是使用 MIT 授權的開放原始碼專案。即使如此，但大多數的人並不會去讀這些原始碼，而且這個專案整個 repo 內的程式也非常複雜，其中包含了 Node.js，Rust，Elixir，以及 Flutter。所以我就跟 AI 一起把整個驗證身分的流程跟相對應的原始碼位置幫大家整理出來，也歡迎大家有什麼建議或修正都可以送 Pull requests 來。

***

**以下為我跟 AI 協作，AI 負責草稿以及對應的 code links，並由我確認內容**

當一個社群服務說「已驗證真人」時，最重要的問題不是它用了哪個 API，而是：平台究竟知道了什麼、保存了什麼，又能把什麼交給其他人？

Elix 的目標不是實名制。它要讓使用者能在需要時證明「這是一位通過特定條件驗證的人」，但不把姓名、身分證字號、護照號碼或出生年月日變成社群平台可以任意查詢的帳號資料。

不過，這不是一句「所有資料都不會傳到後端」就能說清楚的事情。不同驗證方式有不同的資料路徑；尤其是 MobileMoica／TW FidO 與護照 NFC，隱私邊界並不相同。以下是依 Elix 原始碼整理的可檢驗說明。程式碼連結固定指向本文檢查時的 [commit `c117a03`](https://github.com/hcchien/Ansible/tree/c117a03ee9100bfe813d60d4fe17451ee41a6522)，避免日後程式改動使說明失去對照。

## DID：不是姓名，而是使用者可控制的驗證身分

DID（Decentralized Identifier）可先理解成一個可驗證的帳號識別碼，例如 `did:elix:…`。它本身不是法定姓名，也不應包含身分證字號或護照號碼。它所代表的是：某把公開金鑰所對應的身分，能否對一段指定訊息做出有效簽章。

文中的 VC（Verifiable Credential，可驗證憑證）可以想成一張放在自己手機裡的「驗證結果卡」：它可以說明「這個帳號已通過真人驗證」或「持有人已滿十八歲」。其他服務能確認這張卡確實由可信的驗證單位發出、仍然有效，卻不需要看到你的身分證或護照原件。

對使用者而言，這帶來三件事：

1. **同一個人可用自己的 DID 取得 VC。** Issuer 把這張電子證明綁定到 holder DID；日後要出示時，還需要證明自己控制該 DID。
2. **驗證結果可與證件原文分離。** 社群或其他驗證者看到的是「已驗證真人」、「國籍為 TWN」或「已滿十八歲」這類經 Issuer 簽署的聲明，而不是證件影像或證號。
3. **私鑰應留在使用者裝置。** Elix 的 DID 簽章介面明確將私鑰操作放在本機的 Secure Enclave／StrongBox 邊界；伺服器使用公開金鑰驗證簽章。[`did_signer.dart`](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_core/did/lib/src/did_signer.dart#L22-L30)

因此，DID 不是「平台替你保管的帳號密碼」，而是讓使用者在不交出真實姓名的前提下，仍能證明自己有權使用一個驗證結果的工具。

## 兩條資料流，兩種隱私邊界

### 一、MobileMoica／TW FidO：原始身分證字號在驗證當下是必要資料

這一條流程不是 zero-knowledge proof。使用者在 App 輸入身分證字號並同意揭露說明後，App 會向 Elix Issuer 的 `/api/v1/vc/mobilemoica/start` 傳送：holder DID、**原始身分證字號**、同意版本與同意文字雜湊。[App client](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_node/app/lib/services/vc_issuer_client.dart#L190-L205)｜[同意畫面文字](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_node/app/lib/screens/mobilemoica_rp_credential_screen.dart#L12-L21)

Issuer 為何需要原字號？因為 MobileMoica 的票券申請格式本身要求 `id_num`，且 service-provider checksum 的輸入也包含身分證字號。[票券請求與 checksum](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/provider/mobilemoica_rp.go#L548-L591) 因此，目前不能把 App 傳送的原字號簡單替換為 hash；只傳 hash 無法向上游服務發起這個驗證。

資料路徑可概括如下：

```text
使用者輸入身分證字號
  → Elix App
  → Elix Issuer（驗證請求、計算唯一性 commitment）
  → MobileMoica／TW FidO 官方服務（申請 ticket、完成簽章驗證）
  → Elix Issuer（只取得驗證結果）
  → 使用者本機 Wallet（收到 VC）
```

但是 Elix 資料庫並不儲存原始的身分證字號。Issuer 在記憶體中以原字號計算帶有 server-side pepper 的 HMAC-SHA256 commitment，供「同一人不可重複取得有效憑證」使用；HMAC 的 key 不在資料庫中，原值也不會被此函式保存或回傳。[commitment 演算法](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/commitment/commitment.go#L8-L17)｜[`personhood_bindings` 資料表欄位](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/pgstore/pgstore.go#L28-L48)｜[MobileMoica session 寫入](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/provider/postgres_session_store.go#L28-L43)

原始字號在這裡仍是敏感資料，應受到嚴格限制：不得寫進 application log、proxy／APM trace、錯誤訊息、重試佇列或持久 session。這是這條流程必須持續檢驗的隱私承諾，而不是因為資料庫未保存就可以忽略的問題。

### 二、護照 NFC：原始護照資料留在手機，伺服器收到 ZK proof

護照流程的起點是使用者手機透過 NFC 讀取電子護照晶片。用來開啟晶片的 MRZ access data、讀出的 DG1、SOD、護照號碼、出生日期與到期日，都在 App／原生 NFC 程式內處理；Android 路徑還特別避免把另一份 raw MRZ 複製到跨平台邊界。[NFC reader](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_core/vc/lib/src/nfc_passport_reader.dart#L155-L223)｜[護照資料模型](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_core/vc/lib/src/passport_data.dart#L5-L58)

接著，App 先向 Issuer 取得一次性、15 分鐘有效、綁定 DID 的 challenge；伺服器保存的是 challenge ID、DID、**nonce hash**、issuer、scope、電路版本與到期／使用時間，而不是護照資料。[challenge 建立](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/api/passport_challenge.go#L18-L28)｜[PostgreSQL schema](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/pgstore/pgstore.go#L227-L232)

這裡的 challenge 可以想成伺服器臨時發的一次性「通關碼」，不是用來隱藏資料的加密密碼。它是一串隨機值，會指定「哪一個 DID、要向哪個 Issuer、為了什麼用途」提出這一次驗證；15 分鐘後失效，用過即作廢。App 要把它一起納入 proof，讓舊 proof 不能被別人撿去重送，也不能拿來替另一個 DID 申請憑證。伺服器只保存這串通關碼的雜湊值，因此即使資料庫外洩，也不能直接拿出一個尚可使用的 challenge。

App 在本機以 DG1、SOD 和該 challenge 產生 proof，並先做本機驗證；上傳的是 DID、challenge、國籍、ZK proof、verification-key hash 與 holder DID 簽章，不是 MRZ、護照號碼、姓名、出生日期、DG1 或 SOD 原文。[本機 proof 產生](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_core/vc/lib/src/zkp_proof.dart#L110-L287)｜[送出發證請求](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_node/app/lib/screens/credential_issuance_wizard.dart#L688-L754)

這三個名詞可以這樣理解：

* **ZK proof（零知識證明）**：把「我持有一份有效護照，而且符合這次驗證的條件」做成可檢查的數學證明；驗證方可確認結果，但不用取得護照的原始欄位。
* **verification-key hash**：一串用來辨識這次 proof／驗證設定的指紋值，讓 Issuer 與 verifier 能確認雙方檢查的是同一份、未被替換的證明材料；它不是護照號碼或姓名。
* **holder DID 簽章**：使用者以自己手機裡 DID 的私鑰，對這次申請內容簽名。它證明送出 proof 的人確實控制這個 DID，避免有人把別人的有效 proof 綁到自己的帳號。

資料路徑如下：

```text
電子護照晶片（MRZ、DG1、SOD）
  → 僅使用者手機：讀取、驗證、產生 ZK proof
  → Elix Issuer：DID + challenge + 國籍 + ZK proof + DID 簽章
  → ZKPassport verifier：驗證 proof，回傳最小結果
  → Elix Issuer：簽發 VC
  → 使用者本機 Wallet：保存 VC
```

Issuer 驗證 challenge、DID 控制權與 ZK proof 後，才依驗證結果簽發 VC；程式也明確註記，現行 Wallet 不再上傳 client 自算的護照號碼 hash，而由 verifier 從 proof 推導可供重複防護使用的結果。[Issuer 驗證與發證](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/api/handler.go#L680-L822)

這並非「零傳輸」：ZK proof、DID、challenge 和國籍仍會傳輸，而且 ZK proof 應被當成敏感驗證材料妥善保護。但它的重點是，Issuer 不需取得護照的原始識別資料，就能驗證所需條件。

## 最後留下什麼？

### 使用者裝置的 Wallet

Wallet 保存 Issuer 簽署後的 VC，以及使用它所需的本機 DID／金鑰材料。護照 VC 可分為「已驗證真人」、國籍與（若適用）「已滿十八歲」等可分開出示的聲明。[護照 VC 的最小 claims](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/vc/issuer.go#L154-L218) VC 不含護照號碼、身分證字號、姓名或出生日期。

### Elix Issuer 的持久資料庫

Issuer 不是只留下一個模糊的 boolean。為了提供撤銷與「一人一份有效高保證憑證」的防護，它至少會保存下列資料：

| 資料 | 用途 | 是否原始證件資料 |
| --- | --- | -------- |
| `credential_id`、`holder_did`、`status` | 對應憑證、持有人與撤銷狀態 | 否 |
| `commitment`／`national_id_hash`／`passport_number_hash` | 帶 pepper 或 verifier 產生的重複防護值 | 否；但仍屬高敏感、可關聯資料 |
| `offer_id`、`state`、DID、`subject_commitment`、到期與 consumed 狀態 | MobileMoica 單次驗證、重放防護與發證 | 否 |
| challenge ID、DID、nonce hash、issuer、scope、電路版本、到期與 consumed 時間 | 護照 proof 的單次 challenge／防重放 | 否 |

表中前兩類的 schema 可直接見於 [`personhood_bindings`](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/pgstore/pgstore.go#L28-L48)，短期 provider session 可見於 [`provider_auth_sessions` 與 `provider_verified_sessions`](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/pgstore/pgstore.go#L49-L75)。session 清除程式會按到期時間處理資料，但實際保存天數仍須由部署設定與營運政策公開說明、持續驗證。[cleanup 實作](https://github.com/hcchien/Ansible/blob/c117a03ee9100bfe813d60d4fe17451ee41a6522/ansible_issuer/go/internal/provider/postgres_session_store.go#L188-L199)

## 
