package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ehgks0000/rf-server-go/utils"
	"github.com/gin-gonic/gin"
	opensearch "github.com/opensearch-project/opensearch-go/v2"
	opensearchapi "github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

type getRandomPostPublic struct {
	Take int     `form:"take,default=21" binding:"min=0,max=21"`
	Sex  *string `form:"sex" binding:"omitempty,oneof=Male Female"`
}

const ES_POSTS_INDEX = "posts"

func (server *Server) GetRandomPostPublic(c *gin.Context) {
	var req getRandomPostPublic
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	dateRange := utils.GetDateRange(time.Now())

	params := SearchParams{
		Index:     ES_POSTS_INDEX,
		Size:      req.Take,
		Sex:       (*Sex)(req.Sex),
		DateRange: dateRange,
	}

	data, err := getRandomPostsElasticSearchService(params, server.es)

	if err != nil {
		server.logger.Fatalf("Opensearch failed: %v", err)
	}

	defer data.Body.Close() // 항상 response body를 close해야 합니다.

	bodyBytes, err := io.ReadAll(data.Body)
	if err != nil {
		server.logger.Fatalf("Error reading body: %v", err)
	}

	var esResponse ElasticSearchResponse
	err = json.Unmarshal(bodyBytes, &esResponse) // data가 []byte 타입이라고 가정합니다.
	if err != nil {
		server.logger.Fatalf("Failed to unmarshal elasticsearch response: %v", err)
	}

	// _source들만 추출합니다.
	var ids []int

	// 각 hit에서 _source의 ID를 추출하고 슬라이스에 추가합니다.
	for _, hit := range esResponse.Hits.Hits {
		ids = append(ids, hit.Source.ID)
	}

	posts := server.getRandomPostsPublicSql(ids)

	sex := "None"
	if req.Sex != nil {
		sex = *req.Sex
	}

	response := gin.H{
		"take":  req.Take,
		"sex":   sex,
		"size":  len(posts),
		"posts": posts,
	}

	// sources를 JSON으로 반환합니다.
	c.JSON(http.StatusOK, response)
}

type Image struct {
	URL string `json:"url"`
}

type Profile struct {
	Avatar *string `json:"avartar"`
	// Avatar sql.NullString `json:"avartar,omitempty"`
}

type User struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Profile Profile `json:"profile"`
}

type Count struct {
	Favorites int `json:"favorites"`
}

type Post struct {
	IsFavorite bool    `json:"isFavorite"`
	IsFollow   bool    `json:"isFollow"`
	IsScrap    bool    `json:"isScrap"`
	ID         int     `json:"id"`
	CreatedAt  string  `json:"createdAt"`
	Images     []Image `json:"images"`
	User       User    `json:"user"`
	Count      Count   `json:"_count"`
}

func (server *Server) getRandomPostsPublicSql(postIDs []int) []Post {

	args := make([]interface{}, len(postIDs))
	for i, id := range postIDs {
		args[i] = id
	}
	placeholders := strings.Repeat("?,", len(postIDs)-1) + "?"
	sqlQuery := fmt.Sprintf(`
			SELECT
				posts.id,
				posts.created_at,
				images.url,
				COUNT(favorites.id) AS favoritesCount,
				users.id,
				users.name,
				profiles.avartar
			FROM
				posts
			JOIN
				users ON users.id = posts.user_id
			LEFT JOIN
				profiles ON profiles.user_id = users.id
			LEFT JOIN
				images ON images.post_id = posts.id
			LEFT JOIN
				favorites ON favorites.post_id = posts.id
			WHERE
				posts.id IN (%s)
			GROUP BY
				posts.id,
				images.id,
				users.id,
				profiles.id
			ORDER BY
				posts.created_at DESC;
		`, placeholders)

	rows, err := server.db.Query(sqlQuery, args...)

	if err != nil {
		server.logger.Println("Database query failed: ", err)
		return nil
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {

		var (
			postID    int
			createdAt string
			imageURL  string
			count     int
			userID    int
			userName  string
			avatar    sql.NullString // avatar가 NULL 일 수 있기 때문에
		)

		err := rows.Scan(&postID, &createdAt, &imageURL, &count, &userID, &userName, &avatar)
		if err != nil {
			server.logger.Fatal(err)
			return nil
		}

		var avatarPtr *string
		if avatar.Valid {
			avatarPtr = &avatar.String
		}

		post := Post{
			ID:        postID,
			CreatedAt: createdAt,
			Images: []Image{
				{URL: imageURL},
			},
			User: User{
				ID:   userID,
				Name: userName,
				Profile: Profile{
					Avatar: avatarPtr, // sql.NullString의 값 가져오기
				},
			},
			Count: Count{
				Favorites: count,
			},
		}

		posts = append(posts, post)
	}

	// 에러 체크
	if err := rows.Err(); err != nil {
		server.logger.Fatal(err)
		return nil
	}

	return posts
}

type Sex string
type Order string

type ElasticSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source struct {
				ID int `json:"id"`
			} `json:"_source"`
			// Source map[string]interface{} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type SearchParams struct {
	Index     string
	Size      int
	Sex       *Sex
	DateRange utils.DateRange
	Order     *Order
	Heights   []int
	Weights   []int
}

func getRandomPostsElasticSearchService(params SearchParams, client *opensearch.Client) (*opensearchapi.Response, error) {
	var sort []map[string]interface{}
	if params.Order != nil {
		if *params.Order == "recent" {
			sort = append(sort, map[string]interface{}{
				"created_at": map[string]string{
					"order": "desc",
				},
			})
		} else if *params.Order == "best" {
			sort = append(sort, map[string]interface{}{
				"like_count": map[string]string{
					"order": "desc",
				},
				"created_at": map[string]string{
					"order": "desc",
				},
			})
		}
	}

	var filterQueries []map[string]interface{}

	filterQueries = append(filterQueries, map[string]interface{}{
		"term": map[string]int{
			"is_public": 1,
		},
	})

	filterQueries = append(filterQueries, map[string]interface{}{
		"range": map[string]map[string]string{
			"created_at": {
				// "gte": params.DateRange["gte"],
				// "lte": params.DateRange["lte"],
				"gte": params.DateRange.Gte,
				"lte": params.DateRange.Lte,
			},
		},
	})

	if params.Heights != nil && len(params.Heights) == 2 {
		filterQueries = append(filterQueries, map[string]interface{}{
			"range": map[string]map[string]int{
				"height": {
					"gte": params.Heights[0],
					"lte": params.Heights[1],
				},
			},
		})
	}

	if params.Weights != nil && len(params.Weights) == 2 {
		filterQueries = append(filterQueries, map[string]interface{}{
			"range": map[string]map[string]int{
				"weight": {
					"gte": params.Weights[0],
					"lte": params.Weights[1],
				},
			},
		})
	}

	if params.Sex != nil {
		filterQueries = append(filterQueries, map[string]interface{}{
			"term": map[string]string{
				"sex.keyword": string(*params.Sex),
			},
		})
	}

	query := map[string]interface{}{
		"size": params.Size,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": filterQueries,
				"must": map[string]interface{}{
					"function_score": map[string]interface{}{
						"functions": []map[string]interface{}{
							{
								"random_score": map[string]interface{}{},
							},
						},
					},
				},
				"must_not": []map[string]interface{}{
					{
						"exists": map[string]string{
							"field": "deleted_at",
						},
					},
				},
			},
		},
	}

	if len(sort) > 0 {
		query["sort"] = sort
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	return client.Search(
		client.Search.WithContext(context.Background()),
		client.Search.WithIndex(params.Index),
		client.Search.WithBody(bytes.NewReader(queryJSON)),
	)
}
