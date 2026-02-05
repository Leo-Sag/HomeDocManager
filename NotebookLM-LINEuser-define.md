# Project Specification: Family RAG Bot Implementation

## 1. Overview

既存のGoogle Cloud Run (Go言語) 上で稼働しているLINE Botに対し、Googleドキュメントを知識源とするRAG（検索拡張生成）機能を追加実装する。
ユーザー（家族）のLINE IDに基づいてコンテキストを切り替え、契約書や生活情報に関する質問に回答する機能を構築する。

## 2. Technical Stack

* **Platform:** Google Cloud Run
* **Language:** Go (Golang) 1.22+
* **Infrastructure:**
  * **LINE Messaging API:** 既存のBot機能を拡張
  * **Google Docs API:** ドキュメントのテキスト抽出に使用
  * **Google Gemini API:** `gemini-1.5-flash` (Pay-as-you-go / Paid Tier) を使用

## 3. Implementation Phases

### Phase 1: Authentication & Configuration

1. **Google Cloud Project Setup**
    * Enable **Google Docs API** and **Google Drive API**.
    * Create a **Service Account (SA)** named `family-bot-doc-reader`.
    * Generate a JSON Key for the SA and set it as an environment variable (e.g., `GOOGLE_APPLICATION_CREDENTIALS` or `GCP_SA_KEY_JSON`).

2. **Gemini API Setup**
    * Use the API Key linked to the paid GCP project to ensure **privacy (no training on data)**.
    * Set environment variable: `GEMINI_API_KEY`.

3. **Data Source Access**
    * Share the target Google Docs (e.g., "2025_life", "2026_money") with the Service Account email address as "Viewer".

### Phase 2: Go Application Logic

Implement the following logic within `main.go` (or split into modules like `pkg/rag`).

#### Step 1: User Context Management

Define a mapping between LINE User IDs and Family Member Names to personalize the context.

```go
// Example Map
var UserMap = map[string]string{
    "Uxxxxxxxx...": "Dad (Leo)",
    "Uyyyyyyyy...": "Mom",
}
#### Step 2: Google Docs Text Extraction
Implement a function to fetch and concatenate text from Google Docs.

Library: google.golang.org/api/docs/v1

Logic:

Authenticate using the Service Account.

Retrieve the document using srv.Documents.Get(docID).Do().

Iterate through doc.Body.Content. Inside each StructuralElement, check Paragraph.

Inside Paragraph, iterate through Elements (ParagraphElement) and extract TextRun.Content.

Note: Ignore non-text elements to avoid runtime errors.

#### Step 3: RAG & Generation (Gemini 1.5 Flash)
Implement the interaction with Gemini.

Library: github.com/google/generative-ai-go/genai

Model: gemini-1.5-flash

Generation Config:

Temperature: 0.0 (Strictly factual)

System Prompt:

"You are a helpful family assistant. Answer the user's question based ONLY on the provided context. The context contains mixed information for the whole family. The current user is [UserName]. Prioritize information relevant to [UserName]. If the answer is not in the context, explicitly state 'Information not found in the documents'."

#### Step 4: LINE Handler Integration
Modify the existing /callback handler.

On receiving a TextMessage:

Identify UserName from UserID.

Call the Docs Fetcher function (fetch text from all target Doc IDs).

Call the Gemini RAG function with (UserQuery, UserName, DocContext).

Reply to LINE with the generated text.

## 4 Security & Privacy Requirements
No Training: Ensure the Gemini API Key is associated with a billed GCP project.

Error Handling: Do not expose raw stack traces to LINE users. Reply with a friendly error message if the backend fails.
