package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/ehgks0000/rf-server-go/utils"
	"github.com/gin-gonic/gin"
	opensearch "github.com/opensearch-project/opensearch-go/v2"
	opensearchapi "github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

type getRandomPostPublic struct {
	Take int    `form:"take,default=21" binding:"min=0,max=21"`
	Sex  string `form:"sex" binding:"oneof=Male Female"`
}

func (server *Server) GetRandomPostPublic(c *gin.Context) {

	var req getRandomPostPublic
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	dateRange := utils.GetDateRange(time.Now())

	index := "posts"
	params := SearchParams{
		Index:     index,
		Size:      req.Take,
		Sex:       (*Sex)(&req.Sex),
		DateRange: dateRange,
		// 다른 필드들도 여기에 설정합니다.
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
	sources := make([]map[string]interface{}, len(esResponse.Hits.Hits))
	for i, hit := range esResponse.Hits.Hits {
		sources[i] = hit.Source
	}

	response := gin.H{
		"take": req.Take,
		"sex":  req.Sex,
		"data": sources,
	}

	// sources를 JSON으로 반환합니다.
	c.JSON(http.StatusOK, response)
}

type Sex string
type Order string

type ElasticSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source map[string]interface{} `json:"_source"`
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
