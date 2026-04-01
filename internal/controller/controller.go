package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

type BlogNotFoundError struct {
	Name string
}

func (e BlogNotFoundError) Error() string {
	return fmt.Sprintf("Blog '%s' not found", e.Name)
}

type BlogAlreadyExistsError struct {
	Field string
	Value string
}

func (e BlogAlreadyExistsError) Error() string {
	return fmt.Sprintf("Blog with %s '%s' already exists", e.Field, e.Value)
}

type ArticleNotFoundError struct {
	ID int64
}

func (e ArticleNotFoundError) Error() string {
	return fmt.Sprintf("Article %d not found", e.ID)
}

func AddBlog(db *storage.Database, name string, url string, feedURL string, scrapeSelector string, topics []string) (model.Blog, error) {
	if existing, err := db.GetBlogByName(name); err != nil {
		return model.Blog{}, err
	} else if existing != nil {
		return model.Blog{}, BlogAlreadyExistsError{Field: "name", Value: name}
	}
	if existing, err := db.GetBlogByURL(url); err != nil {
		return model.Blog{}, err
	} else if existing != nil {
		return model.Blog{}, BlogAlreadyExistsError{Field: "URL", Value: url}
	}
	lowered := make([]string, len(topics))
	for i, t := range topics {
		lowered[i] = strings.ToLower(t)
	}
	blog := model.Blog{
		Name:           name,
		URL:            url,
		FeedURL:        feedURL,
		ScrapeSelector: scrapeSelector,
		Topics:         lowered,
	}
	return db.AddBlog(blog)
}

func RemoveBlog(db *storage.Database, name string) error {
	blog, err := db.GetBlogByName(name)
	if err != nil {
		return err
	}
	if blog == nil {
		return BlogNotFoundError{Name: name}
	}
	_, err = db.RemoveBlog(blog.ID)
	return err
}

func GetArticles(db *storage.Database, showAll bool, blogName string, category string, topics []string) ([]model.Article, map[int64]string, error) {
	var blogIDs []int64
	if blogName != "" {
		blog, err := db.GetBlogByName(blogName)
		if err != nil {
			return nil, nil, err
		}
		if blog == nil {
			return nil, nil, BlogNotFoundError{Name: blogName}
		}
		blogIDs = []int64{blog.ID}
	}
	if len(topics) > 0 {
		topicBlogs, err := db.ListBlogsByTopics(topics)
		if err != nil {
			return nil, nil, err
		}
		ids := blogIDSet(topicBlogs)
		if len(blogIDs) > 0 {
			blogIDs = intersectIDs(blogIDs, ids)
		} else {
			blogIDs = ids
		}
		if len(blogIDs) == 0 {
			return nil, make(map[int64]string), nil
		}
	}
	var categoryPtr *string
	if category != "" {
		categoryPtr = &category
	}
	var allArticles []model.Article
	if len(blogIDs) > 0 {
		for _, id := range blogIDs {
			bid := id
			articles, err := db.ListArticles(!showAll, &bid, categoryPtr)
			if err != nil {
				return nil, nil, err
			}
			allArticles = append(allArticles, articles...)
		}
	} else {
		articles, err := db.ListArticles(!showAll, nil, categoryPtr)
		if err != nil {
			return nil, nil, err
		}
		allArticles = articles
	}
	blogs, err := db.ListBlogs()
	if err != nil {
		return nil, nil, err
	}
	blogNames := make(map[int64]string)
	for _, blog := range blogs {
		blogNames[blog.ID] = blog.Name
	}
	return allArticles, blogNames, nil
}

func MarkArticleRead(db *storage.Database, articleID int64) (model.Article, error) {
	article, err := db.GetArticle(articleID)
	if err != nil {
		return model.Article{}, err
	}
	if article == nil {
		return model.Article{}, ArticleNotFoundError{ID: articleID}
	}
	if !article.IsRead {
		_, err = db.MarkArticleRead(articleID)
		if err != nil {
			return model.Article{}, err
		}
	}
	return *article, nil
}

func MarkAllArticlesRead(db *storage.Database, blogName string, topics []string) ([]model.Article, error) {
	var blogIDs []int64
	if blogName != "" {
		blog, err := db.GetBlogByName(blogName)
		if err != nil {
			return nil, err
		}
		if blog == nil {
			return nil, BlogNotFoundError{Name: blogName}
		}
		blogIDs = []int64{blog.ID}
	}
	if len(topics) > 0 {
		topicBlogs, err := db.ListBlogsByTopics(topics)
		if err != nil {
			return nil, err
		}
		ids := blogIDSet(topicBlogs)
		if len(blogIDs) > 0 {
			blogIDs = intersectIDs(blogIDs, ids)
		} else {
			blogIDs = ids
		}
		if len(blogIDs) == 0 {
			return nil, nil
		}
	}
	var allArticles []model.Article
	if len(blogIDs) > 0 {
		for _, id := range blogIDs {
			bid := id
			articles, err := db.ListArticles(true, &bid, nil)
			if err != nil {
				return nil, err
			}
			allArticles = append(allArticles, articles...)
		}
	} else {
		articles, err := db.ListArticles(true, nil, nil)
		if err != nil {
			return nil, err
		}
		allArticles = articles
	}
	for _, article := range allArticles {
		_, err := db.MarkArticleRead(article.ID)
		if err != nil {
			return nil, err
		}
	}
	return allArticles, nil
}

func MarkArticleUnread(db *storage.Database, articleID int64) (model.Article, error) {
	article, err := db.GetArticle(articleID)
	if err != nil {
		return model.Article{}, err
	}
	if article == nil {
		return model.Article{}, ArticleNotFoundError{ID: articleID}
	}
	if article.IsRead {
		_, err = db.MarkArticleUnread(articleID)
		if err != nil {
			return model.Article{}, err
		}
	}
	return *article, nil
}

func CleanupArticles(db *storage.Database, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	return db.DeleteOldArticles(cutoff)
}

func blogIDSet(blogs []model.Blog) []int64 {
	ids := make([]int64, len(blogs))
	for i, b := range blogs {
		ids[i] = b.ID
	}
	return ids
}

func intersectIDs(a, b []int64) []int64 {
	set := make(map[int64]struct{}, len(b))
	for _, id := range b {
		set[id] = struct{}{}
	}
	var result []int64
	for _, id := range a {
		if _, ok := set[id]; ok {
			result = append(result, id)
		}
	}
	return result
}
