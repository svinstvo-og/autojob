package bot

import (
	"context"
	"fmt"
	"html"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"autojob/internal/config"
	"autojob/internal/db"
	"autojob/internal/freehire"
	"autojob/internal/tailor"

	tele "gopkg.in/telebot.v3"
)

type Bot struct {
	tb       *tele.Bot
	cfg      *config.Config
	store    *db.Store
	fh       *freehire.Client
	tailor   *tailor.Service
	sessions *SessionStore
}

func New(cfg *config.Config, store *db.Store, fh *freehire.Client, tailorSvc *tailor.Service) (*Bot, error) {
	pref := tele.Settings{
		Token:  cfg.TelegramBotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	tb, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("init telegram bot: %w", err)
	}

	b := &Bot{
		tb:       tb,
		cfg:      cfg,
		store:    store,
		fh:       fh,
		tailor:   tailorSvc,
		sessions: NewSessionStore(),
	}

	b.registerHandlers()
	return b, nil
}

func (b *Bot) Start() {
	log.Println("Telegram bot started polling...")
	b.tb.Start()
}

func (b *Bot) Stop() {
	b.tb.Stop()
}

func (b *Bot) registerHandlers() {
	// Strict Telegram ID Whitelist Middleware (FR-1.1)
	b.tb.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			sender := c.Sender()
			if sender == nil || !b.cfg.AllowedTelegramIDs[sender.ID] {
				if sender != nil {
					log.Printf("Unauthorized access attempt from Telegram ID %d (@%s)", sender.ID, sender.Username)
				}
				return nil
			}
			return next(c)
		}
	})

	b.tb.Handle("/start", b.handleHelp)
	b.tb.Handle("/help", b.handleHelp)
	b.tb.Handle("/settings", b.handleSettings)
	b.tb.Handle("/set_city", b.handleSetCity)
	b.tb.Handle("/set_categories", b.handleSetCategories)
	b.tb.Handle("/jobs", func(c tele.Context) error {
		return b.handleSearchJobs(c, "all")
	})
	b.tb.Handle("/unseen", func(c tele.Context) error {
		return b.handleSearchJobs(c, "unseen")
	})
	b.tb.Handle("/tailor", b.handleTailorCommand)

	b.tb.Handle(tele.OnCallback, b.handleCallback)
}

func (b *Bot) handleHelp(c tele.Context) error {
	ctx := context.Background()
	us, err := b.store.GetOrCreateUserSettings(ctx, c.Sender().ID, b.cfg.DefaultCity, b.cfg.DefaultCategories)
	if err != nil {
		log.Printf("GetOrCreateUserSettings error: %v", err)
	}

	msg := fmt.Sprintf(
		"👋 <b>Autojob — Personal Job Search & CV Tailoring</b>\n\n"+
			"<b>Search Commands:</b>\n"+
			"• <code>/unseen [days]</code> — Show unseen jobs from the last N days (default %d)\n"+
			"• <code>/jobs [days]</code> — Show all open jobs (marks already seen with 👀)\n"+
			"• <code>/tailor &lt;public_slug&gt;</code> — Tailor your CV for a specific job slug\n\n"+
			"<b>Configuration Commands:</b>\n"+
			"• <code>/settings</code> — View current city & categories\n"+
			"• <code>/set_city &lt;city&gt;</code> — Set target city (e.g. <code>/set_city Prague</code>)\n"+
			"• <code>/set_categories &lt;cat1,cat2&gt;</code> — Set Freehire categories (e.g. <code>/set_categories backend,software_engineering,devops</code>)\n\n"+
			"<b>Current Settings:</b>\n"+
			"• City: <code>%s</code>\n"+
			"• Categories: <code>%s</code>",
		b.cfg.DefaultLookback,
		html.EscapeString(us.City),
		html.EscapeString(strings.Join(us.Categories, ",")),
	)
	return c.Send(msg, tele.ModeHTML)
}

func (b *Bot) handleSettings(c tele.Context) error {
	ctx := context.Background()
	us, err := b.store.GetOrCreateUserSettings(ctx, c.Sender().ID, b.cfg.DefaultCity, b.cfg.DefaultCategories)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Failed to load settings: %v", err))
	}

	msg := fmt.Sprintf(
		"⚙️ <b>Your Preferences</b>\n\n"+
			"• <b>City</b>: <code>%s</code>\n"+
			"• <b>Categories</b>: <code>%s</code>\n\n"+
			"Update with <code>/set_city Prague</code> or <code>/set_categories backend,software_engineering</code>",
		html.EscapeString(us.City),
		html.EscapeString(strings.Join(us.Categories, ",")),
	)
	return c.Send(msg, tele.ModeHTML)
}

func (b *Bot) handleSetCity(c tele.Context) error {
	city := strings.TrimSpace(c.Message().Payload)
	if city == "" {
		return c.Send("Usage: <code>/set_city Prague</code>", tele.ModeHTML)
	}

	ctx := context.Background()
	if err := b.store.UpdateCity(ctx, c.Sender().ID, city, b.cfg.DefaultCategories); err != nil {
		return c.Send(fmt.Sprintf("❌ Failed to update city: %v", err))
	}
	return c.Send(fmt.Sprintf("✅ City updated to <code>%s</code>", html.EscapeString(city)), tele.ModeHTML)
}

func (b *Bot) handleSetCategories(c tele.Context) error {
	payload := strings.TrimSpace(c.Message().Payload)
	if payload == "" {
		return c.Send(
			"Usage: <code>/set_categories backend,software_engineering,devops</code>",
			tele.ModeHTML,
		)
	}

	rawParts := strings.FieldsFunc(payload, func(r rune) bool {
		return r == ',' || r == ' '
	})

	var categories []string
	var invalid []string
	for _, p := range rawParts {
		cat := strings.ToLower(strings.TrimSpace(p))
		if cat == "" {
			continue
		}
		if !freehire.ValidCategories[cat] {
			invalid = append(invalid, cat)
		} else {
			categories = append(categories, cat)
		}
	}

	if len(invalid) > 0 {
		return c.Send(fmt.Sprintf(
			"❌ Invalid Freehire category: <code>%s</code>\nCommon values: <code>backend, software_engineering, fullstack, frontend, devops, sre, data_engineering, ml_ai, ai_engineering, qa, security, architecture</code>",
			html.EscapeString(strings.Join(invalid, ", ")),
		), tele.ModeHTML)
	}
	if len(categories) == 0 {
		return c.Send("❌ Please specify at least one category.")
	}

	ctx := context.Background()
	if err := b.store.UpdateCategories(ctx, c.Sender().ID, categories, b.cfg.DefaultCity); err != nil {
		return c.Send(fmt.Sprintf("❌ Failed to update categories: %v", err))
	}
	return c.Send(fmt.Sprintf("✅ Categories updated to <code>%s</code>", html.EscapeString(strings.Join(categories, ","))), tele.ModeHTML)
}

func (b *Bot) handleSearchJobs(c tele.Context, mode string) error {
	days := b.cfg.DefaultLookback
	payload := strings.TrimSpace(c.Message().Payload)
	if payload != "" {
		parsed, err := strconv.Atoi(payload)
		if err != nil || parsed <= 0 || parsed > 365 {
			return c.Send("❌ Please provide a valid number of lookback days (1–365), e.g. <code>/unseen 14</code>", tele.ModeHTML)
		}
		days = parsed
	}

	ctx := context.Background()
	us, err := b.store.GetOrCreateUserSettings(ctx, c.Sender().ID, b.cfg.DefaultCity, b.cfg.DefaultCategories)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Database error: %v", err))
	}

	jobs, err := b.fh.SearchJobs(ctx, freehire.SearchParams{
		City:           us.City,
		Categories:     us.Categories,
		OpenWithinDays: days,
		MaxResults:     100,
	})
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Failed to query Freehire API: %v", err))
	}

	if len(jobs) == 0 {
		return c.Send(fmt.Sprintf(
			"No open jobs found in <b>%s</b> for <code>%s</code> within the last <b>%d</b> days.",
			html.EscapeString(us.City),
			html.EscapeString(strings.Join(us.Categories, ",")),
			days,
		), tele.ModeHTML)
	}

	slugs := make([]string, len(jobs))
	for i, j := range jobs {
		slugs[i] = j.PublicSlug
	}

	seenMap, err := b.store.GetSeenMap(ctx, slugs)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Failed to check seen jobs: %v", err))
	}

	var filtered []freehire.Job
	if mode == "unseen" {
		for _, j := range jobs {
			if !seenMap[j.PublicSlug] {
				filtered = append(filtered, j)
			}
		}
		if len(filtered) == 0 {
			return c.Send(fmt.Sprintf(
				"🎉 All <b>%d</b> open jobs in <b>%s</b> (last %d days) have already been seen! Use <code>/jobs %d</code> to browse them anyway.",
				len(jobs),
				html.EscapeString(us.City),
				days,
				days,
			), tele.ModeHTML)
		}
	} else {
		filtered = jobs
	}

	sess := b.sessions.Create(c.Sender().ID, mode, us.City, us.Categories, days, filtered, seenMap)

	// Mark only the 4 jobs on Page 0 as seen
	b.markPageSeen(ctx, sess, 0)

	msgText := FormatJobPage(sess, 0)
	markup := BuildPaginationKeyboard(sess, 0)

	return c.Send(msgText, tele.ModeHTML, tele.NoPreview, markup)
}

func (b *Bot) markPageSeen(ctx context.Context, sess *SearchSession, page int) {
	pageJobs := sess.PageJobs(page)
	if len(pageJobs) == 0 {
		return
	}
	slugs := make([]string, len(pageJobs))
	for i, j := range pageJobs {
		slugs[i] = j.PublicSlug
	}
	if err := b.store.MarkJobsSeen(ctx, slugs); err != nil {
		log.Printf("Failed to mark page %d jobs seen: %v", page, err)
	}
}

func BuildPaginationKeyboard(sess *SearchSession, page int) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	var navRow []tele.Btn

	if page > 0 {
		navRow = append(navRow, markup.Data("⬅️ Previous", "pg", sess.ID, strconv.Itoa(page-1)))
	}
	if page+1 < sess.TotalPages() {
		navRow = append(navRow, markup.Data("Next ➡️", "pg", sess.ID, strconv.Itoa(page+1)))
	}

	tailorBtn := markup.Data("✨ Tailor CV", "tl_menu", sess.ID, strconv.Itoa(page))

	var rows []tele.Row
	if len(navRow) > 0 {
		rows = append(rows, markup.Row(navRow...))
	}
	rows = append(rows, markup.Row(tailorBtn))
	markup.Inline(rows...)
	return markup
}

func BuildTailorSelectionKeyboard(sess *SearchSession, page int) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	pageJobs := sess.PageJobs(page)

	var numBtns []tele.Btn
	for i := range pageJobs {
		absIdx := page*PageSize + i
		label := fmt.Sprintf("%d", i+1)
		numBtns = append(numBtns, markup.Data(label, "tl_pick", sess.ID, strconv.Itoa(page), strconv.Itoa(absIdx)))
	}

	cancelBtn := markup.Data("❌ Cancel", "tl_back", sess.ID, strconv.Itoa(page))

	markup.Inline(
		markup.Row(numBtns...),
		markup.Row(cancelBtn),
	)
	return markup
}

func (b *Bot) handleCallback(c tele.Context) error {
	cb := c.Callback()
	if cb == nil {
		return nil
	}

	// Telebot formats callback data with "\f<unique>|<arg1>|<arg2>..."
	raw := strings.TrimPrefix(cb.Data, "\f")
	parts := strings.Split(raw, "|")
	if len(parts) < 3 {
		return c.Respond()
	}

	action := parts[0]
	sessionID := parts[1]
	page, err := strconv.Atoi(parts[2])
	if err != nil {
		return c.Respond()
	}

	sess, ok := b.sessions.Get(sessionID)
	if !ok {
		_ = c.Respond(&tele.CallbackResponse{
			Text:      "⚠️ This search session expired. Please run /unseen or /jobs again.",
			ShowAlert: true,
		})
		return nil
	}

	switch action {
	case "pg":
		if page < 0 || page >= sess.TotalPages() {
			return c.Respond()
		}
		b.markPageSeen(context.Background(), sess, page)
		msgText := FormatJobPage(sess, page)
		markup := BuildPaginationKeyboard(sess, page)
		_ = c.Respond()
		return c.Edit(msgText, tele.ModeHTML, tele.NoPreview, markup)

	case "tl_menu":
		_ = c.Respond(&tele.CallbackResponse{
			Text: "Select which job (1–4) on this page to tailor your CV for:",
		})
		markup := BuildTailorSelectionKeyboard(sess, page)
		return c.Edit(markup)

	case "tl_back":
		_ = c.Respond()
		markup := BuildPaginationKeyboard(sess, page)
		return c.Edit(markup)

	case "tl_pick":
		if len(parts) < 4 {
			return c.Respond()
		}
		absIdx, err := strconv.Atoi(parts[3])
		if err != nil || absIdx < 0 || absIdx >= len(sess.Jobs) {
			return c.Respond()
		}
		job := sess.Jobs[absIdx]

		// Restore the pagination buttons on the job list message
		markup := BuildPaginationKeyboard(sess, page)
		_ = c.Edit(markup)
		_ = c.Respond(&tele.CallbackResponse{
			Text: fmt.Sprintf("Tailoring CV for %s...", job.Company),
		})

		b.startAsyncTailoring(c.Chat(), job)
		return nil
	}

	return c.Respond()
}

func (b *Bot) handleTailorCommand(c tele.Context) error {
	slug := strings.TrimSpace(c.Message().Payload)
	if slug == "" {
		return c.Send("Usage: <code>/tailor &lt;public_slug&gt;</code>", tele.ModeHTML)
	}

	job, ok := b.sessions.GetJobBySlug(slug)
	if !ok || strings.TrimSpace(job.Description) == "" {
		fetched, err := b.fh.GetJob(context.Background(), slug)
		if err != nil {
			return c.Send(fmt.Sprintf("❌ Could not find job <code>%s</code>: %v", html.EscapeString(slug), err), tele.ModeHTML)
		}
		job = *fetched
		b.sessions.CacheJob(job)
	}

	b.startAsyncTailoring(c.Chat(), job)
	return nil
}

// startAsyncTailoring runs Gemini CLI and PDF compilation in a background goroutine (NFR-3.1)
// so the Telegram bot stays responsive.
func (b *Bot) startAsyncTailoring(chat *tele.Chat, job freehire.Job) {
	statusMsg, err := b.tb.Send(
		chat,
		fmt.Sprintf(
			"⏳ <b>Tailoring CV</b> for <b>%s</b> at <b>%s</b> (<code>%s</code>)...\n<i>Running Gemini CLI & compiling PDF in background...</i>",
			html.EscapeString(job.Title),
			html.EscapeString(job.Company),
			html.EscapeString(job.PublicSlug),
		),
		tele.ModeHTML,
	)
	if err != nil {
		log.Printf("Failed to send status message: %v", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		// Ensure we have the full description if missing
		if strings.TrimSpace(job.Description) == "" {
			if fetched, err := b.fh.GetJob(ctx, job.PublicSlug); err == nil {
				job = *fetched
			}
		}

		res, err := b.tailor.GenerateTailoredCV(ctx, job)
		if err != nil {
			log.Printf("Tailoring failed for %s: %v", job.PublicSlug, err)
			errText := fmt.Sprintf(
				"❌ <b>CV Tailoring failed</b> for <code>%s</code>:\n<pre>%s</pre>",
				html.EscapeString(job.PublicSlug),
				html.EscapeString(err.Error()),
			)
			if statusMsg != nil {
				_, _ = b.tb.Edit(statusMsg, errText, tele.ModeHTML)
			} else {
				_, _ = b.tb.Send(chat, errText, tele.ModeHTML)
			}
			return
		}

		// Record in PostgreSQL tailored_history (NFR-2.1)
		if err := b.store.SaveTailoredHistory(ctx, job.PublicSlug, res.MDPath, res.PDFPath); err != nil {
			log.Printf("Failed to save tailored_history for %s: %v", job.PublicSlug, err)
		}

		if statusMsg != nil {
			_, _ = b.tb.Edit(
				statusMsg,
				fmt.Sprintf(
					"✅ <b>Tailored CV ready</b> for <b>%s</b> — <b>%s</b> (<code>%s</code>)",
					html.EscapeString(job.Title),
					html.EscapeString(job.Company),
					html.EscapeString(job.PublicSlug),
				),
				tele.ModeHTML,
			)
		}

		mdDoc := &tele.Document{
			File:     tele.FromDisk(res.MDPath),
			FileName: filepath.Base(res.MDPath),
			Caption:  fmt.Sprintf("📄 Tailored CV (Markdown) — %s @ %s", job.Title, job.Company),
		}
		if _, err := b.tb.Send(chat, mdDoc); err != nil {
			log.Printf("Failed to send Markdown document: %v", err)
		}

		pdfDoc := &tele.Document{
			File:     tele.FromDisk(res.PDFPath),
			FileName: filepath.Base(res.PDFPath),
			Caption:  fmt.Sprintf("📑 Tailored CV (PDF) — %s @ %s", job.Title, job.Company),
		}
		if _, err := b.tb.Send(chat, pdfDoc); err != nil {
			log.Printf("Failed to send PDF document: %v", err)
		}
	}()
}
