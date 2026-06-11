package sqlite

func (d *Database) seedDefaultSources() error {
	defaultSources := []Source{
		{
			ID:              "aihot",
			Name:            "AI HOT 热点",
			Type:            "rss",
			URL:             "https://aihot.virxact.com/feed/all.xml",
			Schedule:        "0 8,12,18,22 * * *",
			Enabled:         1,
			DefaultCategory: "auto",
			Category:        "AI",
			Config:          `{"full_content":false,"fetch_full_via_scrape":false}`,
		},
		{
			ID:              "hn",
			Name:            "Hacker News",
			Type:            "rss",
			URL:             "https://hnrss.org/frontpage",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "auto",
			Category:        "科技新闻",
			Config:          `{"full_content":false,"fetch_full_via_scrape":false}`,
		},
		{
			ID:              "hn_best",
			Name:            "Hacker News Best",
			Type:            "rss",
			URL:             "https://hnrss.org/best",
			Schedule:        "0 9 * * *",
			Enabled:         1,
			DefaultCategory: "auto",
			Category:        "科技新闻",
			Config:          `{"full_content":false}`,
		},
		// 国内新闻
		{
			ID:              "chinanews_scroll",
			Name:            "国内新闻-中新网",
			Type:            "rss",
			URL:             "https://www.chinanews.com.cn/rss/scroll-news.xml",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国内新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_newscn",
			Name:            "国内新闻-新华网",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/newscn/whxw",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国内新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_cctv",
			Name:            "国内新闻-央视",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/weixin/cctvnewscenter",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国内新闻",
			Config:          `{"full_content":false}`,
		},
		// 科技新闻
		{
			ID:              "rsshub_36kr",
			Name:            "科技新闻-36氪",
			Type:            "rss",
			URL:             "https://rsshub.app/36kr/newsflashes",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "科技新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "ifanr",
			Name:            "科技新闻-爱范儿",
			Type:            "rss",
			URL:             "https://www.ifanr.com/feed",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "科技新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "solidot",
			Name:            "科技新闻-Solidot",
			Type:            "rss",
			URL:             "https://www.solidot.org/index.rss",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "科技新闻",
			Config:          `{"full_content":false}`,
		},
		// 财经新闻
		{
			ID:              "chinanews_finance",
			Name:            "财经新闻-中新网",
			Type:            "rss",
			URL:             "https://www.chinanews.com.cn/rss/finance.xml",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "财经新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_fortune",
			Name:            "财经新闻-财富中文",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/fortunechina",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "财经新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_tmt",
			Name:            "财经新闻-钛媒体",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/tmtpost",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "财经新闻",
			Config:          `{"full_content":false}`,
		},
		// 国际新闻
		{
			ID:              "chinanews_world",
			Name:            "国际新闻-中新网",
			Type:            "rss",
			URL:             "https://www.chinanews.com.cn/rss/world.xml",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国际新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_bbc",
			Name:            "国际新闻-BBC",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/bbc/cn",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国际新闻",
			Config:          `{"full_content":false}`,
		},
		{
			ID:              "anyfeeder_zaobao",
			Name:            "国际新闻-联合早报",
			Type:            "rss",
			URL:             "https://plink.anyfeeder.com/zaobao/realtime/world",
			Schedule:        "0 */2 * * *",
			Enabled:         1,
			DefaultCategory: "article",
			Category:        "国际新闻",
			Config:          `{"full_content":false}`,
		},
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO sources (id, name, type, url, schedule, enabled, default_category, config, category, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET category = CASE WHEN sources.category = 'General' OR sources.category = '' THEN excluded.category ELSE sources.category END
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, src := range defaultSources {
		_, err := stmt.Exec(src.ID, src.Name, src.Type, src.URL, src.Schedule, src.Enabled, src.DefaultCategory, src.Config, src.Category)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
