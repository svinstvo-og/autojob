# Autojob — Personal Job Search & CV Tailoring Service

A self-hosted personal automation helper to monitor open job postings from freehire.me, track seen postings, and generate position-tailored CVs (Markdown and PDF) using a local Gemini CLI instance.

## 1. Scope & Constraints (MVP)
- **Target Geography**: Prague only (`cities=Prague`). Remote is explicitly out of scope for MVP.
- **Specialization**: Filtered using Freehire's predefined `category` facet (e.g., `backend`, `software_engineering`, `devops`).
- **Interaction Model**: On-demand polling via Telegram bot commands (no background notification daemons needed for MVP).
- **Access Control**: Strict Telegram ID whitelist.

---

## 2. Functional Requirements (FR)

### 2.1 Configuration & Access
- **FR-1.1 Whitelist Authorization**: The bot checks incoming Telegram user IDs against an environment whitelist. Unauthorized users are ignored or rejected.
- **FR-1.2 User Preferences**: User can set their target configuration once (stored in PostgreSQL):
  - `city`: Set to `Prague` (default).
  - `categories`: One or more selected Freehire categories (e.g., `backend,software_engineering`).
  - `lookback_days`: Number of days to look back for recent postings (`open_within_days`, e.g., 7 or 14 days).

### 2.2 Job Querying & Tracking
- **FR-2.1 Query Open Jobs**:
  - User can request open jobs matching their configured category and city.
  - The service queries Freehire's `GET /agent/jobs/search` with `cities=Prague&category=<categories>&open_within_days=<days>&description_format=markdown`.
- **FR-2.2 Query Unseen (New) Jobs**:
  - The service compares returned job `public_slug`s against the PostgreSQL `seen_jobs` table.
  - Returns only jobs where `public_slug` has not yet been recorded.
- **FR-2.3 Automatic Seen Marking**:
  - Jobs returned to the user have their `public_slug` automatically inserted into PostgreSQL with a timestamp to prevent duplicate results on subsequent `/unseen` calls.
- **FR-2.4 Display & Message Formatting**:
  - Display each job with its Title, external `url`, and Description.
  - If a job description (or batch of jobs) exceeds Telegram's 4,096-character limit, the service splits the text across sequential messages without truncation.
  - Each job posting displays its `public_slug` or an inline button `[Tailor CV]` to easily trigger tailoring.

### 2.3 CV Tailoring Pipeline
- **FR-3.1 Base CV Storage**: The user maintains a master, truthful CV file in Markdown (`base_cv.md`) placed manually on the server.
- **FR-3.2 Tailoring Execution**:
  - Triggered by command (e.g., `/tailor <public_slug>`) or inline button.
  - If full description is not in memory, the service fetches it via Freehire's `/jobs/{public_slug}` or `/agent/jobs/search`.
  - Service invokes local Gemini CLI with user's tailoring prompt, feeding:
    1. Base CV content (`base_cv.md`).
    2. Target job details (title, company, full description).
  - Gemini CLI outputs a tailored CV in Markdown format.
- **FR-3.3 PDF Generation & Delivery**:
  - A local rendering script compiles the generated `.md` into a `.pdf`.
  - The bot sends both the `.md` and `.pdf` files directly to the user in Telegram.

---

## 3. Non-Functional Requirements (NFR)

### 3.1 External API Integration
- **NFR-1.1 Source**: Freehire public API (`https://freehire.me/api/v1`).
- **NFR-1.2 Endpoint**: `GET /agent/jobs/search` for search queries to receive untruncated markdown descriptions.
- **NFR-1.3 Identification**: Use Freehire's unique `public_slug` as the primary key for tracking seen/processed jobs.

### 3.2 Storage (PostgreSQL)
- **NFR-2.1 Schema**:
  - `user_settings`: `(telegram_id BIGINT PRIMARY KEY, city TEXT, categories TEXT[], lookback_days INT)`
  - `seen_jobs`: `(public_slug TEXT PRIMARY KEY, first_seen_at TIMESTAMP WITH TIME ZONE DEFAULT NOW())`
  - `tailored_history`: `(id SERIAL PRIMARY KEY, public_slug TEXT, md_path TEXT, pdf_path TEXT, created_at TIMESTAMP)`

### 3.3 Execution & Dependencies
- **NFR-3.1 Non-Blocking LLM Calls**: Gemini CLI calls must be executed asynchronously so the Telegram bot remains responsive while the LLM generates the CV.
- **NFR-3.2 Local Tooling Requirements**:
  - Python / Node.js runtime.
  - Local PostgreSQL instance or container.
  - Gemini CLI installed and authenticated on the host machine.
  - Local Markdown-to-PDF compiler script.
