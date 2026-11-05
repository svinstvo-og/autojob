package bot

import (
	"fmt"
	"html"
	"strings"

	"autojob/internal/freehire"
)

// FormatJobPage formats up to 4 jobs on the requested page in Telegram HTML mode.
func FormatJobPage(sess *SearchSession, page int) string {
	jobs := sess.PageJobs(page)
	totalPages := sess.TotalPages()

	modeTitle := "Open Jobs"
	if sess.Mode == "unseen" {
		modeTitle = "Unseen Jobs"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"🔎 <b>%s in %s</b> (last %d days)\n",
		html.EscapeString(modeTitle),
		html.EscapeString(sess.City),
		sess.Days,
	))
	sb.WriteString(fmt.Sprintf(
		"🏷 Categories: <code>%s</code>\n",
		html.EscapeString(strings.Join(sess.Categories, ", ")),
	))
	sb.WriteString(fmt.Sprintf(
		"📄 Page <b>%d/%d</b> (%d total matching)\n\n",
		page+1, totalPages, len(sess.Jobs),
	))

	for i, j := range jobs {
		seenBadge := ""
		if sess.Mode == "all" && sess.SeenMap[j.PublicSlug] {
			seenBadge = " 👀"
		}

		sb.WriteString(fmt.Sprintf(
			"<b>%d. %s</b> — <b>%s</b>%s\n",
			i+1,
			html.EscapeString(j.Title),
			html.EscapeString(j.Company),
			seenBadge,
		))

		var metaParts []string
		if j.Location != "" {
			metaParts = append(metaParts, html.EscapeString(j.Location))
		}
		if j.WorkMode != "" {
			metaParts = append(metaParts, html.EscapeString(j.WorkMode))
		}
		if j.Enrichment.Seniority != "" {
			metaParts = append(metaParts, html.EscapeString(j.Enrichment.Seniority))
		}
		if sal := formatSalary(j.Enrichment); sal != "" {
			metaParts = append(metaParts, html.EscapeString(sal))
		}
		if len(metaParts) > 0 {
			sb.WriteString(fmt.Sprintf("   📍 %s\n", strings.Join(metaParts, " • ")))
		}

		if len(j.Skills) > 0 {
			maxSkills := 6
			skills := j.Skills
			suffix := ""
			if len(skills) > maxSkills {
				skills = skills[:maxSkills]
				suffix = "…"
			}
			sb.WriteString(fmt.Sprintf("   🛠 %s%s\n", html.EscapeString(strings.Join(skills, ", ")), suffix))
		}

		sb.WriteString(fmt.Sprintf("   🆔 <code>%s</code>\n", html.EscapeString(j.PublicSlug)))
		if j.URL != "" {
			sb.WriteString(fmt.Sprintf("   🔗 <a href=\"%s\">View Job Posting</a>\n", html.EscapeString(j.URL)))
		}
		if i < len(jobs)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatSalary(e freehire.Enrichment) string {
	if e.SalaryMin == nil && e.SalaryMax == nil {
		return ""
	}
	cur := e.SalaryCurrency
	if cur == "" {
		cur = "EUR"
	}
	if e.SalaryMin != nil && e.SalaryMax != nil {
		return fmt.Sprintf("%d–%d %s", *e.SalaryMin, *e.SalaryMax, cur)
	}
	if e.SalaryMin != nil {
		return fmt.Sprintf("from %d %s", *e.SalaryMin, cur)
	}
	return fmt.Sprintf("up to %d %s", *e.SalaryMax, cur)
}
