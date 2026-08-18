package api

// Catálogo de chat dos provedores (IDs oficiais). compatible fica livre.
// GLM: docs.bigmodel.cn — glm-4-flash saiu da tabela; o default é o Flash gratuito atual.

type llmModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func llmModelCatalog() map[string][]llmModelOption {
	return map[string][]llmModelOption{
		"glm": {
			{ID: "glm-4.7-flash", Label: "GLM-4.7 Flash (gratuito)"},
			{ID: "glm-4-flash-250414", Label: "GLM-4-Flash-250414 (gratuito)"},
			{ID: "glm-4.5-flash", Label: "GLM-4.5 Flash (sai de linha)"},
			{ID: "glm-4.7", Label: "GLM-4.7"},
			{ID: "glm-4.6", Label: "GLM-4.6"},
			{ID: "glm-4.5-air", Label: "GLM-4.5 Air"},
			{ID: "glm-5-turbo", Label: "GLM-5 Turbo"},
			{ID: "glm-5", Label: "GLM-5"},
			{ID: "glm-5.1", Label: "GLM-5.1"},
			{ID: "glm-5.2", Label: "GLM-5.2"},
			{ID: "glm-5.3", Label: "GLM-5.3"},
		},
		"openai": {
			{ID: "gpt-4o-mini", Label: "GPT-4o mini"},
			{ID: "gpt-4.1-mini", Label: "GPT-4.1 mini"},
			{ID: "gpt-4.1", Label: "GPT-4.1"},
			{ID: "gpt-4o", Label: "GPT-4o"},
		},
		"anthropic": {
			{ID: "claude-3-5-haiku-20241022", Label: "Claude 3.5 Haiku"},
			{ID: "claude-haiku-4-5", Label: "Claude Haiku 4.5"},
			{ID: "claude-sonnet-4-5", Label: "Claude Sonnet 4.5"},
			{ID: "claude-opus-4-6", Label: "Claude Opus 4.6"},
		},
	}
}
