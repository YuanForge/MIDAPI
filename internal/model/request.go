package model

// ImageRequest 图片生成接口的标准入参（平台统一格式）。
type ImageRequest struct {
	Model       string                 `json:"model" binding:"required"`
	Prompt      string                 `json:"prompt" binding:"required"`
	Size        string                 `json:"size"`
	AspectRatio string                 `json:"aspect_ratio"`
	ReferImages []string               `json:"refer_images"`
	N           int                    `json:"n"`
	Extra       map[string]interface{} `json:"-"`
}

// VideoRequest 视频生成接口的标准入参（平台统一格式）。
type VideoRequest struct {
	Model       string                 `json:"model" binding:"required"`
	Prompt      string                 `json:"prompt" binding:"required"`
	Size        string                 `json:"size"`
	AspectRatio string                 `json:"aspect_ratio"`
	Duration    string                 `json:"duration"`
	ReferImages []string               `json:"refer_images"`
	ReferVideos []string               `json:"refer_videos"`
	Extra       map[string]interface{} `json:"-"`
}

// AudioRequest 音频生成/语音合成接口的标准入参。
type AudioRequest struct {
	Model    string                 `json:"model" binding:"required"`
	Input    string                 `json:"input"`
	Voice    string                 `json:"voice"`
	Duration int                    `json:"duration"`
	Extra    map[string]interface{} `json:"-"`
}

// ToMap 将 ImageRequest 序列化为 map，供 billing 和 JS 脚本使用。
func (r *ImageRequest) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"model":        r.Model,
		"prompt":       r.Prompt,
		"size":         r.Size,
		"aspect_ratio": r.AspectRatio,
		"refer_images": r.ReferImages,
		"n":            r.N,
	}
	for k, v := range r.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return m
}

// ToMap 将 VideoRequest 序列化为 map，供 billing 和 JS 脚本使用。
func (r *VideoRequest) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"model":        r.Model,
		"prompt":       r.Prompt,
		"size":         r.Size,
		"aspect_ratio": r.AspectRatio,
		"duration":     r.Duration,
		"refer_images": r.ReferImages,
		"refer_videos": r.ReferVideos,
	}
	for k, v := range r.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return m
}

// ToMap 将 AudioRequest 序列化为 map。
func (r *AudioRequest) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"model":    r.Model,
		"input":    r.Input,
		"voice":    r.Voice,
		"duration": r.Duration,
	}
	for k, v := range r.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return m
}

// MusicRequest Suno 音乐生成接口的标准入参（平台统一格式）。
type MusicRequest struct {
	Model                string                 `json:"model" binding:"required"`
	InputType            string                 `json:"input_type"`
	MVVersion            string                 `json:"mv_version"`
	MakeInstrumental     interface{}            `json:"make_instrumental"`
	GptDescriptionPrompt string                 `json:"gpt_description_prompt"`
	Prompt               string                 `json:"prompt"`
	Tags                 string                 `json:"tags"`
	Title                string                 `json:"title"`
	ContinueClipID       string                 `json:"continue_clip_id"`
	ContinueAt           string                 `json:"continue_at"`
	CoverClipID          string                 `json:"cover_clip_id"`
	Task                 string                 `json:"task"`
	MetadataParams       map[string]interface{} `json:"metadata_params"`
	CallbackURL          string                 `json:"callback_url"`
	Extra                map[string]interface{} `json:"-"`
}

// ToMap 将 MusicRequest 序列化为 map，供 billing 和 JS 脚本使用。
func (r *MusicRequest) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"model":                  r.Model,
		"input_type":             r.InputType,
		"mv_version":             r.MVVersion,
		"make_instrumental":      r.MakeInstrumental,
		"gpt_description_prompt": r.GptDescriptionPrompt,
		"prompt":                 r.Prompt,
		"tags":                   r.Tags,
		"title":                  r.Title,
		"continue_clip_id":       r.ContinueClipID,
		"continue_at":            r.ContinueAt,
		"cover_clip_id":          r.CoverClipID,
		"task":                   r.Task,
		"metadata_params":        r.MetadataParams,
		"callback_url":           r.CallbackURL,
	}
	for k, v := range r.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return m
}
