package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
)

func TestDatabaseCreatesFileAndCRUD(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	if blog.ID == 0 {
		t.Fatal("expected blog ID")
	}

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if fetched == nil || fetched.Name != "Test" {
		t.Fatalf("unexpected blog: %+v", fetched)
	}

	articles := []model.Article{
		{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"},
		{BlogID: blog.ID, Title: "Two", URL: "https://example.com/2"},
	}
	count, err := db.AddArticlesBulk(articles)
	if err != nil {
		t.Fatalf("add articles bulk: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 articles, got %d", count)
	}

	list, err := db.ListArticles(false, nil, nil)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(list))
	}

	ok, err := db.MarkArticleRead(list[0].ID)
	if err != nil || !ok {
		t.Fatalf("mark read: %v", err)
	}

	updated, err := db.GetArticle(list[0].ID)
	if err != nil {
		t.Fatalf("get article: %v", err)
	}
	if updated == nil || !updated.IsRead {
		t.Fatalf("expected article read: %+v", updated)
	}

	now := time.Now()
	if err := db.UpdateBlogLastScanned(blog.ID, now); err != nil {
		t.Fatalf("update last scanned: %v", err)
	}

	deleted, err := db.RemoveBlog(blog.ID)
	if err != nil {
		t.Fatalf("remove blog: %v", err)
	}
	if !deleted {
		t.Fatalf("expected blog removal")
	}
}

func TestGetExistingArticleURLs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	existing, err := db.GetExistingArticleURLs([]string{"https://example.com/1", "https://example.com/2"})
	if err != nil {
		t.Fatalf("get existing: %v", err)
	}
	if _, ok := existing["https://example.com/1"]; !ok {
		t.Fatalf("expected existing url")
	}
	if _, ok := existing["https://example.com/2"]; ok {
		t.Fatalf("did not expect url")
	}
}

func TestDatabaseForeignKeyEnforced(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if _, err := db.AddArticle(model.Article{BlogID: 9999, Title: "Orphan", URL: "https://example.com/orphan"}); err == nil {
		t.Fatalf("expected foreign key error for missing blog")
	}
}

func TestBlogOptionalFieldsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected blog")
	}
	if fetched.FeedURL != "" || fetched.ScrapeSelector != "" {
		t.Fatalf("expected empty optional fields: %+v", fetched)
	}
}

func TestBlogTimeRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	now := time.Date(2025, 1, 2, 3, 4, 5, 6, time.UTC)
	blog, err := db.AddBlog(model.Blog{
		Name:        "Test",
		URL:         "https://example.com",
		LastScanned: &now,
	})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if fetched == nil || fetched.LastScanned == nil {
		t.Fatalf("expected last scanned")
	}
	if !fetched.LastScanned.Equal(now) {
		t.Fatalf("expected last scanned %s, got %s", now.Format(time.RFC3339Nano), fetched.LastScanned.Format(time.RFC3339Nano))
	}
}

func TestArticleTimeRoundTripAndNilDiscoveredDate(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	published := time.Date(2024, 12, 31, 23, 59, 59, 123, time.UTC)
	article, err := db.AddArticle(model.Article{
		BlogID:        blog.ID,
		Title:         "Title",
		URL:           "https://example.com/1",
		PublishedDate: &published,
	})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	fetched, err := db.GetArticle(article.ID)
	if err != nil {
		t.Fatalf("get article: %v", err)
	}
	if fetched == nil || fetched.PublishedDate == nil {
		t.Fatalf("expected published date")
	}
	if !fetched.PublishedDate.Equal(published) {
		t.Fatalf("expected published date %s, got %s", published.Format(time.RFC3339Nano), fetched.PublishedDate.Format(time.RFC3339Nano))
	}
	if fetched.DiscoveredDate != nil {
		t.Fatalf("expected discovered date nil when not set")
	}
}

func TestListArticlesFiltersAndOrdering(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blogA, err := db.AddBlog(model.Blog{Name: "A", URL: "https://a.example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	blogB, err := db.AddBlog(model.Blog{Name: "B", URL: "https://b.example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	first, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "Old", URL: "https://a.example.com/old", DiscoveredDate: &t1})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	second, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "New", URL: "https://a.example.com/new", DiscoveredDate: &t3})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: blogB.ID, Title: "Other", URL: "https://b.example.com/1", DiscoveredDate: &t2})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	if _, err := db.MarkArticleRead(first.ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	all, err := db.ListArticles(false, nil, nil)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 articles, got %d", len(all))
	}
	if all[0].ID != second.ID {
		t.Fatalf("expected newest article first")
	}

	unread, err := db.ListArticles(true, nil, nil)
	if err != nil {
		t.Fatalf("list unread: %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("expected 2 unread articles, got %d", len(unread))
	}

	blogID := blogB.ID
	filtered, err := db.ListArticles(false, &blogID, nil)
	if err != nil {
		t.Fatalf("list by blog: %v", err)
	}
	if len(filtered) != 1 || filtered[0].BlogID != blogB.ID {
		t.Fatalf("expected one article for blog B")
	}
}

func TestBulkInsertDuplicateRollbackAndEmpty(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	if count, err := db.AddArticlesBulk(nil); err != nil || count != 0 {
		t.Fatalf("expected empty bulk insert to be no-op, got %d, %v", count, err)
	}

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Existing", URL: "https://example.com/existing"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	dupArticles := []model.Article{
		{BlogID: blog.ID, Title: "Dup", URL: "https://example.com/dup"},
		{BlogID: blog.ID, Title: "Dup2", URL: "https://example.com/dup"},
	}
	if _, err := db.AddArticlesBulk(dupArticles); err == nil {
		t.Fatalf("expected bulk insert to fail on duplicate url")
	}

	articles, err := db.ListArticles(false, nil, nil)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected rollback on duplicate, got %d articles", len(articles))
	}
}

func TestLookupHelpers(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if blog, err := db.GetBlogByName("missing"); err != nil || blog != nil {
		t.Fatalf("expected missing blog by name")
	}
	if blog, err := db.GetBlogByURL("https://missing.example.com"); err != nil || blog != nil {
		t.Fatalf("expected missing blog by url")
	}

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	article, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	if found, err := db.GetArticleByURL(article.URL); err != nil || found == nil {
		t.Fatalf("expected article by url")
	}
	if exists, err := db.ArticleExists(article.URL); err != nil || !exists {
		t.Fatalf("expected article to exist")
	}
	if exists, err := db.ArticleExists("https://example.com/missing"); err != nil || exists {
		t.Fatalf("expected missing article to not exist")
	}
}

func TestListStringHelpers(t *testing.T) {
	if result := listToString(nil); result != nil {
		t.Fatalf("expected nil for empty categories, got %v", result)
	}
	if result := listToString([]string{}); result != nil {
		t.Fatalf("expected nil for empty slice, got %v", result)
	}
	s := listToString([]string{"AI", "Security"})
	if s == nil || *s != "AI,Security" {
		t.Fatalf("expected 'AI,Security', got %v", s)
	}

	if result := listFromString(nil); result != nil {
		t.Fatalf("expected nil for nil string, got %v", result)
	}
	empty := ""
	if result := listFromString(&empty); result != nil {
		t.Fatalf("expected nil for empty string, got %v", result)
	}
	cats := "AI,Security"
	result := listFromString(&cats)
	if len(result) != 2 || result[0] != "AI" || result[1] != "Security" {
		t.Fatalf("expected [AI Security], got %v", result)
	}
}

func TestCategoriesStorageAndFiltering(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	// Article with categories
	a1, err := db.AddArticle(model.Article{
		BlogID:     blog.ID,
		Title:      "AI Post",
		URL:        "https://example.com/ai",
		Categories: []string{"AI", "Machine Learning"},
	})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	// Article with different categories
	_, err = db.AddArticle(model.Article{
		BlogID:     blog.ID,
		Title:      "Security Post",
		URL:        "https://example.com/sec",
		Categories: []string{"Security"},
	})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	// Article with no categories
	_, err = db.AddArticle(model.Article{
		BlogID: blog.ID,
		Title:  "Plain Post",
		URL:    "https://example.com/plain",
	})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	// Verify categories round-trip
	fetched, err := db.GetArticle(a1.ID)
	if err != nil {
		t.Fatalf("get article: %v", err)
	}
	if len(fetched.Categories) != 2 || fetched.Categories[0] != "AI" || fetched.Categories[1] != "Machine Learning" {
		t.Fatalf("expected categories [AI Machine Learning], got %v", fetched.Categories)
	}

	// Filter by exact category
	cat := "AI"
	filtered, err := db.ListArticles(false, nil, &cat)
	if err != nil {
		t.Fatalf("filter by category: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Title != "AI Post" {
		t.Fatalf("expected 1 AI article, got %d", len(filtered))
	}

	// Case-insensitive filter
	catLower := "ai"
	filtered, err = db.ListArticles(false, nil, &catLower)
	if err != nil {
		t.Fatalf("filter by lowercase category: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Title != "AI Post" {
		t.Fatalf("expected case-insensitive match, got %d articles", len(filtered))
	}

	// Case-insensitive: uppercase query matches lowercase stored
	_, err = db.AddArticle(model.Article{
		BlogID:     blog.ID,
		Title:      "Lowercase Category Post",
		URL:        "https://example.com/lower",
		Categories: []string{"security", "devops"},
	})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	catUpper := "Security"
	filtered, err = db.ListArticles(false, nil, &catUpper)
	if err != nil {
		t.Fatalf("filter by uppercase category: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 security articles (case-insensitive), got %d", len(filtered))
	}

	// Mixed case query
	catMixed := "sEcUrItY"
	filtered, err = db.ListArticles(false, nil, &catMixed)
	if err != nil {
		t.Fatalf("filter by mixed case category: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 security articles (mixed case), got %d", len(filtered))
	}

	// Should NOT match substring - "AI" should not match "FAIR"
	_, err = db.AddArticle(model.Article{
		BlogID:     blog.ID,
		Title:      "Fair Post",
		URL:        "https://example.com/fair",
		Categories: []string{"FAIR"},
	})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	filtered, err = db.ListArticles(false, nil, &cat)
	if err != nil {
		t.Fatalf("filter by category after FAIR added: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected AI filter to NOT match FAIR, got %d articles", len(filtered))
	}

	// No category filter returns all
	all, err := db.ListArticles(false, nil, nil)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 articles, got %d", len(all))
	}
}

func TestCategoriesBulkInsert(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	articles := []model.Article{
		{BlogID: blog.ID, Title: "Post 1", URL: "https://example.com/1", Categories: []string{"Go", "Testing"}},
		{BlogID: blog.ID, Title: "Post 2", URL: "https://example.com/2", Categories: []string{"Rust"}},
		{BlogID: blog.ID, Title: "Post 3", URL: "https://example.com/3"},
	}
	count, err := db.AddArticlesBulk(articles)
	if err != nil {
		t.Fatalf("bulk insert: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3, got %d", count)
	}

	cat := "Go"
	filtered, err := db.ListArticles(false, nil, &cat)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Title != "Post 1" {
		t.Fatalf("expected 1 Go article, got %d", len(filtered))
	}
}

func TestDeleteOldArticles(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	old := time.Now().AddDate(0, 0, -400)
	recent := time.Now().AddDate(0, 0, -10)

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Old", URL: "https://example.com/old", DiscoveredDate: &old})
	if err != nil {
		t.Fatalf("add old article: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Recent", URL: "https://example.com/recent", DiscoveredDate: &recent})
	if err != nil {
		t.Fatalf("add recent article: %v", err)
	}

	cutoff := time.Now().AddDate(0, 0, -365)
	deleted, err := db.DeleteOldArticles(cutoff)
	if err != nil {
		t.Fatalf("delete old articles: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	all, err := db.ListArticles(false, nil, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 || all[0].Title != "Recent" {
		t.Fatalf("expected only recent article, got %d", len(all))
	}
}

func TestDeleteOldArticlesNoneToDelete(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	recent := time.Now()
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Fresh", URL: "https://example.com/fresh", DiscoveredDate: &recent})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	cutoff := time.Now().AddDate(0, 0, -365)
	deleted, err := db.DeleteOldArticles(cutoff)
	if err != nil {
		t.Fatalf("delete old articles: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted, got %d", deleted)
	}
}

func TestBlogTopicsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com", Topics: []string{"go", "security"}})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if len(fetched.Topics) != 2 || fetched.Topics[0] != "go" || fetched.Topics[1] != "security" {
		t.Fatalf("expected topics [go security], got %v", fetched.Topics)
	}

	// No topics
	blog2, err := db.AddBlog(model.Blog{Name: "Plain", URL: "https://plain.example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	fetched2, err := db.GetBlog(blog2.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if fetched2.Topics != nil {
		t.Fatalf("expected nil topics, got %v", fetched2.Topics)
	}
}

func TestListBlogsByTopics(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	_, err = db.AddBlog(model.Blog{Name: "GoBlog", URL: "https://go.example.com", Topics: []string{"go", "programming"}})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = db.AddBlog(model.Blog{Name: "SecurityBlog", URL: "https://sec.example.com", Topics: []string{"security"}})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = db.AddBlog(model.Blog{Name: "NullBlog", URL: "https://null.example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	// Single topic filter
	goBlogs, err := db.ListBlogsByTopics([]string{"go"})
	if err != nil {
		t.Fatalf("list by topic: %v", err)
	}
	if len(goBlogs) != 1 || goBlogs[0].Name != "GoBlog" {
		t.Fatalf("expected 1 go blog, got %d", len(goBlogs))
	}

	// Case-insensitive
	goBlogs2, err := db.ListBlogsByTopics([]string{"GO"})
	if err != nil {
		t.Fatalf("list by topic uppercase: %v", err)
	}
	if len(goBlogs2) != 1 {
		t.Fatalf("expected case-insensitive match, got %d", len(goBlogs2))
	}

	// No match
	empty, err := db.ListBlogsByTopics([]string{"rust"})
	if err != nil {
		t.Fatalf("list by topic no match: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 blogs, got %d", len(empty))
	}

	// Substring should NOT match - "go" should not match "golang"
	_, err = db.AddBlog(model.Blog{Name: "GolangBlog", URL: "https://golang.example.com", Topics: []string{"golang"}})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	goBlogs3, err := db.ListBlogsByTopics([]string{"go"})
	if err != nil {
		t.Fatalf("list by topic after golang added: %v", err)
	}
	if len(goBlogs3) != 1 {
		t.Fatalf("expected go filter to NOT match golang, got %d blogs", len(goBlogs3))
	}

	// Multiple topics — should match go AND security blogs
	multi, err := db.ListBlogsByTopics([]string{"go", "security"})
	if err != nil {
		t.Fatalf("list by multiple topics: %v", err)
	}
	if len(multi) != 2 {
		t.Fatalf("expected 2 blogs for go+security, got %d", len(multi))
	}

	// Empty topics returns all blogs
	all, err := db.ListBlogsByTopics(nil)
	if err != nil {
		t.Fatalf("list by nil topics: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 blogs for nil topics, got %d", len(all))
	}
}

func TestGetStatsEmpty(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.TotalBlogs != 0 || stats.TotalArticles != 0 || stats.ReadArticles != 0 || stats.UnreadArticles != 0 {
		t.Fatalf("expected all zeros, got %+v", stats)
	}
	if len(stats.Topics) != 0 {
		t.Fatalf("expected empty topics, got %v", stats.Topics)
	}
	if stats.OldestArticle != nil || stats.NewestArticle != nil {
		t.Fatalf("expected nil dates on empty db")
	}
	if stats.LastScanTime != nil {
		t.Fatalf("expected nil last scan on empty db")
	}
	if stats.DatabaseSize <= 0 {
		t.Fatalf("expected positive db size, got %d", stats.DatabaseSize)
	}
}

func TestGetStatsPopulated(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	_, err = db.AddBlog(model.Blog{Name: "GoBlog", URL: "https://go.example.com", Topics: []string{"go", "programming"}})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = db.AddBlog(model.Blog{Name: "SecBlog", URL: "https://sec.example.com", Topics: []string{"security", "go"}})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = db.AddBlog(model.Blog{Name: "PlainBlog", URL: "https://plain.example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	goBlog, _ := db.GetBlogByName("GoBlog")
	secBlog, _ := db.GetBlogByName("SecBlog")

	now := time.Now()
	a1, _ := db.AddArticle(model.Article{BlogID: goBlog.ID, Title: "Go1", URL: "https://go.example.com/1", DiscoveredDate: &now})
	_, _ = db.AddArticle(model.Article{BlogID: goBlog.ID, Title: "Go2", URL: "https://go.example.com/2", DiscoveredDate: &now})
	_, _ = db.AddArticle(model.Article{BlogID: secBlog.ID, Title: "Sec1", URL: "https://sec.example.com/1", DiscoveredDate: &now})

	db.MarkArticleRead(a1.ID)

	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}

	if stats.TotalBlogs != 3 {
		t.Fatalf("expected 3 blogs, got %d", stats.TotalBlogs)
	}
	if stats.TotalArticles != 3 {
		t.Fatalf("expected 3 articles, got %d", stats.TotalArticles)
	}
	if stats.ReadArticles != 1 {
		t.Fatalf("expected 1 read, got %d", stats.ReadArticles)
	}
	if stats.UnreadArticles != 2 {
		t.Fatalf("expected 2 unread, got %d", stats.UnreadArticles)
	}
	goTopic := stats.Topics["go"]
	if goTopic == nil || goTopic.Blogs != 2 {
		t.Fatalf("expected 2 blogs with 'go' topic, got %+v", goTopic)
	}
	if goTopic.Total != 3 || goTopic.Read != 1 || goTopic.Unread != 2 {
		t.Fatalf("expected go topic: 3 total, 1 read, 2 unread, got %+v", goTopic)
	}
	progTopic := stats.Topics["programming"]
	if progTopic == nil || progTopic.Blogs != 1 || progTopic.Total != 2 {
		t.Fatalf("expected 1 blog, 2 articles with 'programming' topic, got %+v", progTopic)
	}
	secTopic := stats.Topics["security"]
	if secTopic == nil || secTopic.Blogs != 1 || secTopic.Total != 1 || secTopic.Unread != 1 {
		t.Fatalf("expected 1 blog, 1 unread article with 'security' topic, got %+v", secTopic)
	}
	if stats.OldestArticle == nil || stats.NewestArticle == nil {
		t.Fatalf("expected non-nil article dates")
	}
	if stats.DatabaseSize <= 0 {
		t.Fatalf("expected positive db size, got %d", stats.DatabaseSize)
	}
}
