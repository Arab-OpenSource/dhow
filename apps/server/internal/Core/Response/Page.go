package response

import config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"

func PageParams(page, limit *int) (int, int) {
	p, l := 1, config.DefaultPageLimit
	if page != nil && *page > 0 {
		p = min(*page, config.MaxPage)
	}
	if limit != nil && *limit > 0 {
		l = min(*limit, config.MaxPageLimit)
	}
	return p, l
}
