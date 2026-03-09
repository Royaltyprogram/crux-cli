package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/liushuangls/go-server-template/configs"
)

type AnalyticsStore struct {
	mu sync.RWMutex

	filePath       string
	allowDemoUser  bool
	bootstrapUsers []configs.BootstrapUser
	seq            uint64

	organizations          map[string]*Organization
	users                  map[string]*User
	accessTokens           map[string]*AccessToken
	agents                 map[string]*Agent
	projects               map[string]*Project
	configSnapshots        map[string][]*ConfigSnapshot
	sessionSummaries       map[string][]*SessionSummary
	recommendations        map[string]*Recommendation
	projectRecommendations map[string][]string
	applyOperations        map[string]*ApplyOperation
	audits                 []*AuditEvent
}

type analyticsStoreState struct {
	Seq                    uint64                       `json:"seq"`
	Organizations          map[string]*Organization     `json:"organizations"`
	Users                  map[string]*User             `json:"users"`
	AccessTokens           map[string]*AccessToken      `json:"access_tokens"`
	Agents                 map[string]*Agent            `json:"agents"`
	Projects               map[string]*Project          `json:"projects"`
	ConfigSnapshots        map[string][]*ConfigSnapshot `json:"config_snapshots"`
	SessionSummaries       map[string][]*SessionSummary `json:"session_summaries"`
	Recommendations        map[string]*Recommendation   `json:"recommendations"`
	ProjectRecommendations map[string][]string          `json:"project_recommendations"`
	ApplyOperations        map[string]*ApplyOperation   `json:"apply_operations"`
	Audits                 []*AuditEvent                `json:"audits"`
}

type Organization struct {
	ID   string
	Name string
}

type User struct {
	ID           string
	OrgID        string
	Email        string
	Name         string
	PasswordSalt string
	PasswordHash string
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

type Agent struct {
	ID            string
	OrgID         string
	UserID        string
	DeviceName    string
	Hostname      string
	Platform      string
	CLIVersion    string
	Tools         []string
	ConsentScopes []string
	RegisteredAt  time.Time
}

type Project struct {
	ID             string
	OrgID          string
	AgentID        string
	Name           string
	RepoHash       string
	RepoPath       string
	LanguageMix    map[string]float64
	DefaultTool    string
	LastProfileID  string
	LastIngestedAt *time.Time
	ConnectedAt    time.Time
}

type ConfigSnapshot struct {
	ID                  string
	ProjectID           string
	Tool                string
	ProfileID           string
	Settings            map[string]any
	EnabledMCPCount     int
	HooksEnabled        bool
	InstructionFiles    []string
	ConfigFingerprint   string
	RecentConfigChanges []string
	CapturedAt          time.Time
}

type SessionSummary struct {
	ID         string
	ProjectID  string
	Tool       string
	TokenIn    int
	TokenOut   int
	RawQueries []string
	Timestamp  time.Time
}

type Recommendation struct {
	ID               string
	ProjectID        string
	Kind             string
	Title            string
	Summary          string
	Reason           string
	Explanation      string
	ExpectedBenefit  string
	Risk             string
	ExpectedImpact   string
	Score            float64
	Status           string
	TargetTool       string
	TargetFileHint   string
	ResearchProvider string
	ResearchModel    string
	Evidence         []string
	ChangePlan       []ChangePlanStep
	SettingsUpdates  map[string]any
	CreatedAt        time.Time
}

type ChangePlanStep struct {
	Type            string
	Action          string
	TargetFile      string
	Summary         string
	SettingsUpdates map[string]any
	ContentPreview  string
}

type PatchPreview struct {
	FilePath        string
	Operation       string
	Summary         string
	SettingsUpdates map[string]any
	ContentPreview  string
}

type ApplyOperation struct {
	ID               string
	RecommendationID string
	ProjectID        string
	RequestedBy      string
	Scope            string
	Status           string
	PolicyMode       string
	PolicyReason     string
	ApprovalStatus   string
	Decision         string
	ReviewedBy       string
	ReviewNote       string
	AppliedText      string
	PatchPreview     []PatchPreview
	AppliedFile      string
	AppliedSettings  map[string]any
	Note             string
	RolledBack       bool
	RequestedAt      time.Time
	ReviewedAt       *time.Time
	AppliedAt        *time.Time
}

type AuditEvent struct {
	ID        string
	OrgID     string
	ProjectID string
	Type      string
	Message   string
	CreatedAt time.Time
}

func NewAnalyticsStore(conf *configs.Config) (*AnalyticsStore, error) {
	store := &AnalyticsStore{
		filePath:               conf.App.StorePath,
		allowDemoUser:          conf.AllowsDemoUser(),
		bootstrapUsers:         append([]configs.BootstrapUser(nil), conf.Auth.BootstrapUsers...),
		organizations:          make(map[string]*Organization),
		users:                  make(map[string]*User),
		accessTokens:           make(map[string]*AccessToken),
		agents:                 make(map[string]*Agent),
		projects:               make(map[string]*Project),
		configSnapshots:        make(map[string][]*ConfigSnapshot),
		sessionSummaries:       make(map[string][]*SessionSummary),
		recommendations:        make(map[string]*Recommendation),
		projectRecommendations: make(map[string][]string),
		applyOperations:        make(map[string]*ApplyOperation),
		audits:                 make([]*AuditEvent, 0, 32),
	}
	if err := store.loadFromDisk(); err != nil {
		return nil, err
	}
	if err := store.ensureBootstrapData(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *AnalyticsStore) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s_%06d", prefix, s.seq)
}

func (s *AnalyticsStore) persistLocked() error {
	if s.filePath == "" {
		return nil
	}

	state := analyticsStoreState{
		Seq:                    s.seq,
		Organizations:          s.organizations,
		Users:                  s.users,
		AccessTokens:           s.accessTokens,
		Agents:                 s.agents,
		Projects:               s.projects,
		ConfigSnapshots:        s.configSnapshots,
		SessionSummaries:       s.sessionSummaries,
		Recommendations:        s.recommendations,
		ProjectRecommendations: s.projectRecommendations,
		ApplyOperations:        s.applyOperations,
		Audits:                 s.audits,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}

	tempPath := s.filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, s.filePath)
}

func (s *AnalyticsStore) loadFromDisk() error {
	if s.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var state analyticsStoreState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	s.seq = state.Seq
	s.organizations = ensureMap(state.Organizations)
	s.users = ensureMap(state.Users)
	s.accessTokens = ensureMap(state.AccessTokens)
	s.agents = ensureMap(state.Agents)
	s.projects = ensureMap(state.Projects)
	s.configSnapshots = ensureNestedMap(state.ConfigSnapshots)
	s.sessionSummaries = ensureNestedMap(state.SessionSummaries)
	s.recommendations = ensureMap(state.Recommendations)
	s.projectRecommendations = ensureStringSliceMap(state.ProjectRecommendations)
	s.applyOperations = ensureMap(state.ApplyOperations)
	if state.Audits == nil {
		s.audits = make([]*AuditEvent, 0, 32)
	} else {
		s.audits = state.Audits
	}
	return nil
}

func ensureMap[T any](in map[string]T) map[string]T {
	if in == nil {
		return make(map[string]T)
	}
	return in
}

func ensureNestedMap[T any](in map[string][]T) map[string][]T {
	if in == nil {
		return make(map[string][]T)
	}
	return in
}

func ensureStringSliceMap(in map[string][]string) map[string][]string {
	if in == nil {
		return make(map[string][]string)
	}
	return in
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func cloneFloatMap(input map[string]float64) map[string]float64 {
	if len(input) == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func cloneStringSlice(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	out := make([]string, len(input))
	copy(out, input)
	return out
}

func cloneChangePlanSteps(input []ChangePlanStep) []ChangePlanStep {
	if len(input) == 0 {
		return []ChangePlanStep{}
	}
	out := make([]ChangePlanStep, 0, len(input))
	for _, item := range input {
		out = append(out, ChangePlanStep{
			Type:            item.Type,
			Action:          item.Action,
			TargetFile:      item.TargetFile,
			Summary:         item.Summary,
			SettingsUpdates: cloneAnyMap(item.SettingsUpdates),
			ContentPreview:  item.ContentPreview,
		})
	}
	return out
}

func (s *AnalyticsStore) collapseProjectsLocked() bool {
	orgProjects := make(map[string][]*Project)
	for _, project := range s.projects {
		if project == nil || project.OrgID == "" {
			continue
		}
		orgProjects[project.OrgID] = append(orgProjects[project.OrgID], project)
	}

	modified := false
	for _, projects := range orgProjects {
		if len(projects) <= 1 {
			continue
		}

		canonical := latestProject(projects)
		if canonical == nil {
			continue
		}

		recommendationIDs := make([]string, 0, len(s.projectRecommendations[canonical.ID]))
		seenRecommendationIDs := make(map[string]struct{}, len(s.projectRecommendations[canonical.ID]))
		for _, id := range s.projectRecommendations[canonical.ID] {
			if _, exists := seenRecommendationIDs[id]; exists {
				continue
			}
			seenRecommendationIDs[id] = struct{}{}
			recommendationIDs = append(recommendationIDs, id)
		}

		for _, project := range projects {
			if project == nil || project.ID == canonical.ID {
				continue
			}
			modified = true

			if canonical.LastIngestedAt == nil || (project.LastIngestedAt != nil && project.LastIngestedAt.After(*canonical.LastIngestedAt)) {
				canonical.LastIngestedAt = project.LastIngestedAt
			}
			if canonical.LastProfileID == "" && project.LastProfileID != "" {
				canonical.LastProfileID = project.LastProfileID
			}
			if canonical.DefaultTool == "" && project.DefaultTool != "" {
				canonical.DefaultTool = project.DefaultTool
			}
			if canonical.RepoHash == "" && project.RepoHash != "" {
				canonical.RepoHash = project.RepoHash
			}
			if canonical.RepoPath == "" && project.RepoPath != "" {
				canonical.RepoPath = project.RepoPath
			}
			if len(canonical.LanguageMix) == 0 && len(project.LanguageMix) > 0 {
				canonical.LanguageMix = cloneFloatMap(project.LanguageMix)
			}
			if project.ConnectedAt.After(canonical.ConnectedAt) {
				canonical.AgentID = project.AgentID
				canonical.RepoHash = project.RepoHash
				canonical.RepoPath = project.RepoPath
				canonical.LanguageMix = cloneFloatMap(project.LanguageMix)
				canonical.DefaultTool = project.DefaultTool
				canonical.ConnectedAt = project.ConnectedAt
			}

			for _, snapshot := range s.configSnapshots[project.ID] {
				if snapshot == nil {
					continue
				}
				snapshot.ProjectID = canonical.ID
				s.configSnapshots[canonical.ID] = append(s.configSnapshots[canonical.ID], snapshot)
			}
			delete(s.configSnapshots, project.ID)

			for _, session := range s.sessionSummaries[project.ID] {
				if session == nil {
					continue
				}
				session.ProjectID = canonical.ID
				s.sessionSummaries[canonical.ID] = append(s.sessionSummaries[canonical.ID], session)
			}
			delete(s.sessionSummaries, project.ID)

			for _, recommendationID := range s.projectRecommendations[project.ID] {
				rec := s.recommendations[recommendationID]
				if rec != nil {
					rec.ProjectID = canonical.ID
				}
				if _, exists := seenRecommendationIDs[recommendationID]; exists {
					continue
				}
				seenRecommendationIDs[recommendationID] = struct{}{}
				recommendationIDs = append(recommendationIDs, recommendationID)
			}
			delete(s.projectRecommendations, project.ID)

			for _, rec := range s.recommendations {
				if rec != nil && rec.ProjectID == project.ID {
					rec.ProjectID = canonical.ID
				}
			}

			for _, op := range s.applyOperations {
				if op != nil && op.ProjectID == project.ID {
					op.ProjectID = canonical.ID
				}
			}

			for _, audit := range s.audits {
				if audit != nil && audit.ProjectID == project.ID {
					audit.ProjectID = canonical.ID
				}
			}

			delete(s.projects, project.ID)
		}

		if modified {
			canonical.Name = sharedWorkspaceName
			s.projectRecommendations[canonical.ID] = recommendationIDs
		}
	}

	return modified
}

func latestProject(projects []*Project) *Project {
	if len(projects) == 0 {
		return nil
	}

	sorted := append([]*Project(nil), projects...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ConnectedAt.Equal(sorted[j].ConnectedAt) {
			return sorted[i].ID > sorted[j].ID
		}
		return sorted[i].ConnectedAt.After(sorted[j].ConnectedAt)
	})
	return sorted[0]
}
