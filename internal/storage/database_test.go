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

	first, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "Old", URL: "https://a.example.com/old", DiscoveredDate: &t1})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	second, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "New", URL: "https://a.example.com/new", DiscoveredDate: &t2})
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

func TestCategoriesHelpers(t *testing.T) {
	if result := categoriesToString(nil); result != nil {
		t.Fatalf("expected nil for empty categories, got %v", result)
	}
	if result := categoriesToString([]string{}); result != nil {
		t.Fatalf("expected nil for empty slice, got %v", result)
	}
	s := categoriesToString([]string{"AI", "Security"})
	if s == nil || *s != "AI,Security" {
		t.Fatalf("expected 'AI,Security', got %v", s)
	}

	if result := categoriesFromString(nil); result != nil {
		t.Fatalf("expected nil for nil string, got %v", result)
	}
	empty := ""
	if result := categoriesFromString(&empty); result != nil {
		t.Fatalf("expected nil for empty string, got %v", result)
	}
	cats := "AI,Security"
	result := categoriesFromString(&cats)
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
