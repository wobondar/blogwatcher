package controller

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

func TestAddBlogAndRemoveBlog(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "", nil)
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	if _, err := AddBlog(db, "Test", "https://other.com", "", "", nil); err == nil {
		t.Fatalf("expected duplicate name error")
	}

	if _, err := AddBlog(db, "Other", "https://example.com", "", "", nil); err == nil {
		t.Fatalf("expected duplicate url error")
	}

	if err := RemoveBlog(db, blog.Name); err != nil {
		t.Fatalf("remove blog: %v", err)
	}
}

func TestArticleReadUnread(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "", nil)
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	article, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	read, err := MarkArticleRead(db, article.ID)
	if err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if read.IsRead {
		t.Fatalf("expected original state unread")
	}

	unread, err := MarkArticleUnread(db, article.ID)
	if err != nil {
		t.Fatalf("mark unread: %v", err)
	}
	if !unread.IsRead {
		t.Fatalf("expected original state read")
	}
}

func TestGetArticlesFilters(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "", nil)
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	articles, blogNames, err := GetArticles(db, false, "", "", nil)
	if err != nil {
		t.Fatalf("get articles: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected article")
	}
	if blogNames[blog.ID] != blog.Name {
		t.Fatalf("expected blog name")
	}

	if _, _, err := GetArticles(db, false, "Missing", "", nil); err == nil {
		t.Fatalf("expected blog not found error")
	}
}

func TestCleanupArticles(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "", nil)
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	old := time.Now().AddDate(-2, 0, 0)
	recent := time.Now()
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Old", URL: "https://example.com/old", DiscoveredDate: &old})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Recent", URL: "https://example.com/recent", DiscoveredDate: &recent})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	deleted, err := CleanupArticles(db, 365)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}
}

func TestAddBlogWithTopics(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "", []string{"Go", "SECURITY"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if len(fetched.Topics) != 2 || fetched.Topics[0] != "go" || fetched.Topics[1] != "security" {
		t.Fatalf("expected lowercased topics [go security], got %v", fetched.Topics)
	}
}

func TestGetArticlesFilterByTopic(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := AddBlog(db, "GoBlog", "https://go.example.com", "", "", []string{"go"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = AddBlog(db, "SecBlog", "https://sec.example.com", "", "", []string{"security"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	goBlog, _ := db.GetBlogByName("GoBlog")
	secBlog, _ := db.GetBlogByName("SecBlog")

	_, err = db.AddArticle(model.Article{BlogID: goBlog.ID, Title: "Go Post", URL: "https://go.example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: secBlog.ID, Title: "Sec Post", URL: "https://sec.example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	// Filter by topic
	articles, _, err := GetArticles(db, false, "", "", []string{"go"})
	if err != nil {
		t.Fatalf("get articles by topic: %v", err)
	}
	if len(articles) != 1 || articles[0].Title != "Go Post" {
		t.Fatalf("expected 1 go article, got %d", len(articles))
	}

	// No topic returns all
	all, _, err := GetArticles(db, false, "", "", nil)
	if err != nil {
		t.Fatalf("get all articles: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(all))
	}

	// Non-matching topic returns empty
	empty, _, err := GetArticles(db, false, "", "", []string{"rust"})
	if err != nil {
		t.Fatalf("get articles by missing topic: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 articles, got %d", len(empty))
	}
}

func TestGetArticlesFilterByMultipleTopics(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := AddBlog(db, "GoBlog", "https://go.example.com", "", "", []string{"go"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = AddBlog(db, "SecBlog", "https://sec.example.com", "", "", []string{"security"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = AddBlog(db, "RustBlog", "https://rust.example.com", "", "", []string{"rust"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	goBlog, _ := db.GetBlogByName("GoBlog")
	secBlog, _ := db.GetBlogByName("SecBlog")
	rustBlog, _ := db.GetBlogByName("RustBlog")

	_, _ = db.AddArticle(model.Article{BlogID: goBlog.ID, Title: "Go Post", URL: "https://go.example.com/1"})
	_, _ = db.AddArticle(model.Article{BlogID: secBlog.ID, Title: "Sec Post", URL: "https://sec.example.com/1"})
	_, _ = db.AddArticle(model.Article{BlogID: rustBlog.ID, Title: "Rust Post", URL: "https://rust.example.com/1"})

	// Filter by two topics — should match go and security, not rust
	articles, _, err := GetArticles(db, false, "", "", []string{"go", "security"})
	if err != nil {
		t.Fatalf("get articles by multi-topic: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles (go+security), got %d", len(articles))
	}
}

func TestMarkAllArticlesReadByTopic(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := AddBlog(db, "GoBlog", "https://go.example.com", "", "", []string{"go"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = AddBlog(db, "SecBlog", "https://sec.example.com", "", "", []string{"security"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	goBlog, _ := db.GetBlogByName("GoBlog")
	secBlog, _ := db.GetBlogByName("SecBlog")

	_, _ = db.AddArticle(model.Article{BlogID: goBlog.ID, Title: "Go1", URL: "https://go.example.com/1"})
	_, _ = db.AddArticle(model.Article{BlogID: secBlog.ID, Title: "Sec1", URL: "https://sec.example.com/1"})

	marked, err := MarkAllArticlesRead(db, "", []string{"go"})
	if err != nil {
		t.Fatalf("mark all read by topic: %v", err)
	}
	if len(marked) != 1 || marked[0].Title != "Go1" {
		t.Fatalf("expected 1 marked, got %d", len(marked))
	}

	// Sec article should still be unread
	unread, _, err := GetArticles(db, false, "", "", nil)
	if err != nil {
		t.Fatalf("get articles: %v", err)
	}
	if len(unread) != 1 || unread[0].Title != "Sec1" {
		t.Fatalf("expected 1 unread (Sec1), got %d", len(unread))
	}
}

func TestMultiIDReadUnread(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "", nil)
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	a1, _ := db.AddArticle(model.Article{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"})
	a2, _ := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Two", URL: "https://example.com/2"})
	a3, _ := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Three", URL: "https://example.com/3"})

	// Mark multiple as read
	for _, id := range []int64{a1.ID, a2.ID} {
		_, err := MarkArticleRead(db, id)
		if err != nil {
			t.Fatalf("mark read %d: %v", id, err)
		}
	}

	// Verify
	unread, _, err := GetArticles(db, false, "", "", nil)
	if err != nil {
		t.Fatalf("get articles: %v", err)
	}
	if len(unread) != 1 || unread[0].ID != a3.ID {
		t.Fatalf("expected 1 unread (a3), got %d", len(unread))
	}

	// Mark multiple as unread
	for _, id := range []int64{a1.ID, a2.ID} {
		_, err := MarkArticleUnread(db, id)
		if err != nil {
			t.Fatalf("mark unread %d: %v", id, err)
		}
	}

	all, _, err := GetArticles(db, false, "", "", nil)
	if err != nil {
		t.Fatalf("get articles: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 unread, got %d", len(all))
	}
}

func TestMarkArticleReadNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := MarkArticleRead(db, 9999)
	if err == nil {
		t.Fatalf("expected error for missing article")
	}
	if _, ok := err.(ArticleNotFoundError); !ok {
		t.Fatalf("expected ArticleNotFoundError, got %T", err)
	}
}

func openTestDB(t *testing.T) *storage.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blogwatcher.db")
	db, err := storage.OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}
