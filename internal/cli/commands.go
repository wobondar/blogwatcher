package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Hyaxia/blogwatcher/internal/controller"
	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/scanner"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

func newAddCommand() *cobra.Command {
	var feedURL string
	var scrapeSelector string
	var topics []string

	cmd := &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add a new blog to track.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			url := args[1]
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = controller.AddBlog(db, name, url, feedURL, scrapeSelector, topics)
			if err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Added blog '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&feedURL, "feed-url", "", "RSS/Atom feed URL (auto-discovered if not provided)")
	cmd.Flags().StringVar(&scrapeSelector, "scrape-selector", "", "CSS selector for HTML scraping fallback")
	cmd.Flags().StringSliceVarP(&topics, "topics", "t", nil, "Comma-separated topics for the blog (e.g. --topics \"go,security\")")
	return cmd
}

func newRemoveCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a blog from tracking.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				confirmed, err := confirm(fmt.Sprintf("Remove blog '%s' and all its articles?", name))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			if err := controller.RemoveBlog(db, name); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Removed blog '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newBlogsCommand() *cobra.Command {
	var topics []string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "blogs",
		Short: "List all tracked blogs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			blogs, err := db.ListBlogsByTopics(topics)
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(blogsToJSON(blogs))
			}

			if len(blogs) == 0 {
				fmt.Println("No blogs tracked yet. Use 'blogwatcher add' to add one.")
				return nil
			}
			color.New(color.FgCyan, color.Bold).Printf("Tracked blogs (%d):\n\n", len(blogs))
			for _, blog := range blogs {
				printBlog(blog)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&topics, "topics", "t", nil, "Filter by topics (comma-separated)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func newScanCommand() *cobra.Command {
	var silent bool
	var workers int
	var topics []string

	cmd := &cobra.Command{
		Use:   "scan [blog_name]",
		Short: "Scan blogs for new articles.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			if len(args) == 1 {
				result, err := scanner.ScanBlogByName(db, args[0])
				if err != nil {
					return err
				}
				if result == nil {
					err := fmt.Errorf("Blog '%s' not found", args[0])
					printError(err)
					return markError(err)
				}
				if !silent {
					printScanResult(*result)
				}
			} else {
				blogs, err := db.ListBlogsByTopics(topics)

				if err != nil {
					return err
				}
				if len(blogs) == 0 {
					fmt.Println("No blogs tracked yet. Use 'blogwatcher add' to add one.")
					return nil
				}
				if !silent {
					color.New(color.FgCyan).Printf("Scanning %d blog(s)...\n\n", len(blogs))
				}
				results, err := scanner.ScanBlogs(db, blogs, workers)
				if err != nil {
					return err
				}
				totalNew := 0
				for _, result := range results {
					if !silent {
						printScanResult(result)
					}
					totalNew += result.NewArticles
				}
				if !silent {
					fmt.Println()
					if totalNew > 0 {
						color.New(color.FgGreen, color.Bold).Printf("Found %d new article(s) total!\n", totalNew)
					} else {
						color.New(color.FgYellow).Println("No new articles found.")
					}
				}
			}

			if silent {
				fmt.Println("scan done")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&silent, "silent", "s", false, "Only output 'scan done' when complete")
	cmd.Flags().IntVarP(&workers, "workers", "w", 8, "Number of concurrent workers when scanning all blogs")
	cmd.Flags().StringSliceVarP(&topics, "topics", "t", nil, "Only scan blogs with these topics (comma-separated)")
	return cmd
}

func newArticlesCommand() *cobra.Command {
	var showAll bool
	var blogName string
	var category string
	var topics []string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "articles",
		Short: "List articles.",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			articles, blogNames, err := controller.GetArticles(db, showAll, blogName, category, topics)
			if err != nil {
				printError(err)
				return markError(err)
			}

			if jsonOutput {
				return printJSON(articlesToJSON(articles, blogNames))
			}

			if len(articles) == 0 {
				if showAll {
					fmt.Println("No articles found.")
				} else {
					color.New(color.FgGreen).Println("No unread articles!")
				}
				return nil
			}

			label := "Unread articles"
			if showAll {
				label = "All articles"
			}
			color.New(color.FgCyan, color.Bold).Printf("%s (%d):\n\n", label, len(articles))
			for _, article := range articles {
				printArticle(article, blogNames[article.BlogID])
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all articles (including read)")
	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Filter by blog name")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Filter by category")
	cmd.Flags().StringSliceVarP(&topics, "topics", "t", nil, "Filter by blog topics (comma-separated)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func newReadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <article_id> [article_id...]",
		Short: "Mark one or more articles as read.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			var lastErr error
			for _, arg := range args {
				articleID, err := parseID(arg)
				if err != nil {
					printError(err)
					lastErr = err
					continue
				}
				article, err := controller.MarkArticleRead(db, articleID)
				if err != nil {
					printError(err)
					lastErr = err
					continue
				}
				if article.IsRead {
					fmt.Printf("Article %d is already marked as read.\n", articleID)
				} else {
					color.New(color.FgGreen).Printf("Marked article %d as read\n", articleID)
				}
			}
			if lastErr != nil {
				return markError(lastErr)
			}
			return nil
		},
	}
	return cmd
}

func newReadAllCommand() *cobra.Command {
	var blogName string
	var topics []string
	var yes bool

	cmd := &cobra.Command{
		Use:   "read-all",
		Short: "Mark all unread articles as read.",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			articles, blogNames, err := controller.GetArticles(db, false, blogName, "", topics)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if len(articles) == 0 {
				color.New(color.FgGreen).Println("No unread articles to mark as read.")
				return nil
			}

			if !yes {
				scope := "all blogs"
				if blogName != "" {
					scope = fmt.Sprintf("from '%s'", blogName)
				}
				if len(topics) > 0 {
					scope += fmt.Sprintf(" (topics: %s)", strings.Join(topics, ", "))
				}
				confirmed, err := confirm(fmt.Sprintf("Mark %d article(s) %s as read?", len(articles), scope))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			marked, err := controller.MarkAllArticlesRead(db, blogName, topics)
			if err != nil {
				printError(err)
				return markError(err)
			}

			_ = blogNames
			color.New(color.FgGreen).Printf("Marked %d article(s) as read\n", len(marked))
			return nil
		},
	}

	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Only mark articles from this blog")
	cmd.Flags().StringSliceVarP(&topics, "topics", "t", nil, "Only mark articles from blogs with these topics (comma-separated)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newUnreadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unread <article_id> [article_id...]",
		Short: "Mark one or more articles as unread.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			var lastErr error
			for _, arg := range args {
				articleID, err := parseID(arg)
				if err != nil {
					printError(err)
					lastErr = err
					continue
				}
				article, err := controller.MarkArticleUnread(db, articleID)
				if err != nil {
					printError(err)
					lastErr = err
					continue
				}
				if !article.IsRead {
					fmt.Printf("Article %d is already marked as unread.\n", articleID)
				} else {
					color.New(color.FgGreen).Printf("Marked article %d as unread\n", articleID)
				}
			}
			if lastErr != nil {
				return markError(lastErr)
			}
			return nil
		},
	}
	return cmd
}

func newStatusCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show database summary and statistics.",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			stats, err := db.GetStats()
			if err != nil {
				printError(err)
				return markError(err)
			}

			if jsonOutput {
				return printJSON(statsToJSON(stats))
			}

			color.New(color.FgCyan, color.Bold).Println("BlogWatcher Status")
			fmt.Println()
			fmt.Printf("  Blogs:    %d\n", stats.TotalBlogs)
			fmt.Printf("  Articles: %d total, ", stats.TotalArticles)
			color.New(color.FgYellow).Printf("%d unread", stats.UnreadArticles)
			fmt.Print(", ")
			color.New(color.FgHiBlack).Printf("%d read\n", stats.ReadArticles)

			if stats.OldestArticle != nil {
				fmt.Printf("  Oldest:   %s\n", stats.OldestArticle.Format("2006-01-02"))
			}
			if stats.NewestArticle != nil {
				fmt.Printf("  Newest:   %s\n", stats.NewestArticle.Format("2006-01-02"))
			}
			if stats.LastScanTime != nil {
				fmt.Printf("  Last scan: %s\n", stats.LastScanTime.Format("2006-01-02 15:04"))
			}
			fmt.Printf("  DB size:  %s\n", formatBytes(stats.DatabaseSize))

			if len(stats.Topics) > 0 {
				fmt.Println()
				color.New(color.FgCyan, color.Bold).Println("  Topics:")
				for topic, ts := range stats.Topics {
					fmt.Printf("    %-20s %d blog(s), %d articles (%d unread, %d read)\n",
						topic, ts.Blogs, ts.Total, ts.Unread, ts.Read)
				}
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

type statusJSON struct {
	Blogs        int                       `json:"blogs"`
	Articles     articleStats              `json:"articles"`
	LastScan     *string                   `json:"last_scan,omitempty"`
	DatabaseSize int64                     `json:"database_size_bytes"`
	Topics       map[string]topicStatsJSON `json:"topics,omitempty"`
}

type topicStatsJSON struct {
	Blogs  int `json:"blogs"`
	Total  int `json:"total"`
	Read   int `json:"read"`
	Unread int `json:"unread"`
}

type articleStats struct {
	Total  int     `json:"total"`
	Read   int     `json:"read"`
	Unread int     `json:"unread"`
	Oldest *string `json:"oldest,omitempty"`
	Newest *string `json:"newest,omitempty"`
}

func statsToJSON(stats *storage.Stats) statusJSON {
	var topics map[string]topicStatsJSON
	if len(stats.Topics) > 0 {
		topics = make(map[string]topicStatsJSON, len(stats.Topics))
		for k, v := range stats.Topics {
			topics[k] = topicStatsJSON{Blogs: v.Blogs, Total: v.Total, Read: v.Read, Unread: v.Unread}
		}
	}
	s := statusJSON{
		Blogs: stats.TotalBlogs,
		Articles: articleStats{
			Total:  stats.TotalArticles,
			Read:   stats.ReadArticles,
			Unread: stats.UnreadArticles,
		},
		DatabaseSize: stats.DatabaseSize,
		Topics:       topics,
	}
	if stats.OldestArticle != nil {
		v := stats.OldestArticle.Format(time.RFC3339)
		s.Articles.Oldest = &v
	}
	if stats.NewestArticle != nil {
		v := stats.NewestArticle.Format(time.RFC3339)
		s.Articles.Newest = &v
	}
	if stats.LastScanTime != nil {
		v := stats.LastScanTime.Format(time.RFC3339)
		s.LastScan = &v
	}
	return s
}

func newCleanupCommand() *cobra.Command {
	var days int
	var yes bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove articles older than a specified number of days.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes && !jsonOutput {
				confirmed, err := confirm(fmt.Sprintf("Delete articles older than %d days?", days))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			deleted, err := controller.CleanupArticles(db, days)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if jsonOutput {
				return printJSON(map[string]interface{}{
					"deleted": deleted,
					"days":    days,
				})
			}
			if deleted == 0 {
				fmt.Println("No articles to clean up.")
			} else {
				color.New(color.FgGreen).Printf("Deleted %d article(s)\n", deleted)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&days, "days", "d", 365, "Delete articles older than this many days")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func printScanResult(result scanner.ScanResult) {
	statusColor := color.FgWhite
	if result.NewArticles > 0 {
		statusColor = color.FgGreen
	}
	color.New(color.FgWhite, color.Bold).Printf("  %s\n", result.BlogName)
	if result.Error != "" {
		color.New(color.FgRed).Printf("    Error: %s\n", result.Error)
		return
	}
	if result.Source == "none" {
		color.New(color.FgYellow).Println("    No feed or scraper configured")
		return
	}
	sourceLabel := "HTML"
	if result.Source == "rss" {
		sourceLabel = "RSS"
	}
	fmt.Printf("    Source: %s | Found: %d | ", sourceLabel, result.TotalFound)
	color.New(statusColor).Printf("New: %d\n", result.NewArticles)
}

func printBlog(blog model.Blog) {
	color.New(color.FgWhite, color.Bold).Printf("  %s\n", blog.Name)
	fmt.Printf("    URL: %s\n", blog.URL)
	if blog.FeedURL != "" {
		fmt.Printf("    Feed: %s\n", blog.FeedURL)
	}
	if blog.ScrapeSelector != "" {
		fmt.Printf("    Selector: %s\n", blog.ScrapeSelector)
	}
	if len(blog.Topics) > 0 {
		fmt.Printf("    Topics: %s\n", strings.Join(blog.Topics, ", "))
	}
	if blog.LastScanned != nil {
		fmt.Printf("    Last scanned: %s\n", blog.LastScanned.Format("2006-01-02 15:04"))
	}
	fmt.Println()
}

type articleJSON struct {
	ID            int64    `json:"id"`
	BlogName      string   `json:"blog_name"`
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	PublishedDate *string  `json:"published_date,omitempty"`
	DiscoveredDate *string `json:"discovered_date,omitempty"`
	IsRead        bool     `json:"is_read"`
	Categories    []string `json:"categories,omitempty"`
}

type blogJSON struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	FeedURL        string   `json:"feed_url,omitempty"`
	ScrapeSelector string   `json:"scrape_selector,omitempty"`
	Topics         []string `json:"topics,omitempty"`
	LastScanned    *string  `json:"last_scanned,omitempty"`
}

func articlesToJSON(articles []model.Article, blogNames map[int64]string) []articleJSON {
	result := make([]articleJSON, 0, len(articles))
	for _, a := range articles {
		j := articleJSON{
			ID:         a.ID,
			BlogName:   blogNames[a.BlogID],
			Title:      a.Title,
			URL:        a.URL,
			IsRead:     a.IsRead,
			Categories: a.Categories,
		}
		if a.PublishedDate != nil {
			s := a.PublishedDate.Format(time.RFC3339)
			j.PublishedDate = &s
		}
		if a.DiscoveredDate != nil {
			s := a.DiscoveredDate.Format(time.RFC3339)
			j.DiscoveredDate = &s
		}
		result = append(result, j)
	}
	return result
}

func blogsToJSON(blogs []model.Blog) []blogJSON {
	result := make([]blogJSON, 0, len(blogs))
	for _, b := range blogs {
		j := blogJSON{
			ID:             b.ID,
			Name:           b.Name,
			URL:            b.URL,
			FeedURL:        b.FeedURL,
			ScrapeSelector: b.ScrapeSelector,
			Topics:         b.Topics,
		}
		if b.LastScanned != nil {
			s := b.LastScanned.Format(time.RFC3339)
			j.LastScanned = &s
		}
		result = append(result, j)
	}
	return result
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printArticle(article model.Article, blogName string) {
	status := color.New(color.FgYellow).Sprint("[new]")
	if article.IsRead {
		status = color.New(color.FgHiBlack).Sprint("[read]")
	}
	idStr := color.New(color.FgCyan).Sprintf("[%d]", article.ID)
	fmt.Printf("  %s %s %s\n", idStr, status, article.Title)
	fmt.Printf("       Blog: %s\n", blogName)
	fmt.Printf("       URL: %s\n", article.URL)
	if article.PublishedDate != nil {
		fmt.Printf("       Published: %s\n", article.PublishedDate.Format("2006-01-02"))
	}
	if len(article.Categories) > 0 {
		fmt.Printf("       Categories: %s\n", strings.Join(article.Categories, ", "))
	}
	fmt.Println()
}

func printError(err error) {
	color.New(color.FgRed).Printf("Error: %s\n", err.Error())
}

func parseID(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid article id: %s", value)
	}
	return parsed, nil
}

func formatBytes(b int64) string {
	const kb = 1024
	const mb = kb * 1024
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func confirm(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

func init() {
	cobra.EnableCommandSorting = false
	cobra.AddTemplateFunc("now", func() string { return time.Now().Format(time.RFC3339) })
}
