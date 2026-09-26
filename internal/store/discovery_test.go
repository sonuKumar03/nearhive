package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func forEachStore(t *testing.T, fn func(t *testing.T, s Store)) {
	t.Run("MockStore", func(t *testing.T) {
		fn(t, NewMockStore())
	})

	t.Run("PostgresStore", func(t *testing.T) {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			dsn = "postgres://postgres:postgres@localhost:5432/nearhive?sslmode=disable"
		}
		ps, err := NewPostgresStore(dsn)
		if err != nil {
			t.Skip("Postgres not available, skipping PostgresStore test")
			return
		}
		defer ps.Close()
		if err := ps.db.Ping(); err != nil {
			t.Skip("Postgres ping failed, skipping PostgresStore test")
			return
		}
		fn(t, ps)
	})
}

func TestDiscoveryStore_CreateListCancel(t *testing.T) {
	// Verify constant values required by specification
	assert.Equal(t, model.PresenceType("confirmed_office"), model.PresenceTypeConfirmedOffice)
	assert.Equal(t, model.PresenceType("probable_office"), model.PresenceTypeProbableOffice)
	assert.Equal(t, model.PresenceType("job_location_only"), model.PresenceTypeJobLocationOnly)

	assert.Equal(t, model.WorkArrangement("in_office"), model.WorkArrangementInOffice)
	assert.Equal(t, model.WorkArrangement("hybrid"), model.WorkArrangementHybrid)
	assert.Equal(t, model.WorkArrangement("remote"), model.WorkArrangementRemote)
	assert.Equal(t, model.WorkArrangement("unknown"), model.WorkArrangementUnknown)

	forEachStore(t, func(t *testing.T, s Store) {
		ctx := context.Background()

		userA := &model.User{
			ID:       uuid.New(),
			Email:    fmt.Sprintf("usera-%s@example.com", uuid.New().String()[:8]),
			Password: "hash",
		}
		userB := &model.User{
			ID:       uuid.New(),
			Email:    fmt.Sprintf("userb-%s@example.com", uuid.New().String()[:8]),
			Password: "hash",
		}
		require.NoError(t, s.CreateUser(ctx, userA))
		require.NoError(t, s.CreateUser(ctx, userB))

		jobA := &model.DiscoveryJob{
			UserID:   userA.ID,
			Lat:      12.9716,
			Lng:      77.5946,
			RadiusKM: 10.0,
		}

		err := s.CreateDiscoveryJob(ctx, jobA)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, jobA.ID)
		assert.Equal(t, model.DiscoveryStatusPending, jobA.Status)

		// User scoping: User A can get jobA, User B cannot
		fetchedA, err := s.GetDiscoveryJob(ctx, jobA.ID, userA.ID)
		require.NoError(t, err)
		assert.Equal(t, jobA.ID, fetchedA.ID)
		assert.Equal(t, userA.ID, fetchedA.UserID)

		_, err = s.GetDiscoveryJob(ctx, jobA.ID, userB.ID)
		assert.ErrorIs(t, err, ErrNotFound, "User B must not be able to read User A's job")

		// User scoping in listing: User A sees jobA, User B sees none
		listA, err := s.ListDiscoveryJobs(ctx, userA.ID, 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, listA)
		foundJobA := false
		for _, j := range listA {
			if j.ID == jobA.ID {
				foundJobA = true
				break
			}
		}
		assert.True(t, foundJobA, "User A should see jobA in list")

		listB, err := s.ListDiscoveryJobs(ctx, userB.ID, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, listB, "User B should have no jobs listed")

		// User scoping: User B cannot cancel User A's job
		err = s.CancelDiscoveryJob(ctx, jobA.ID, userB.ID)
		assert.ErrorIs(t, err, ErrNotFound, "User B must not be able to cancel User A's job")

		// Valid status transitions: Pending -> Cancelled
		err = s.CancelDiscoveryJob(ctx, jobA.ID, userA.ID)
		require.NoError(t, err)

		fetchedAAfterCancel, err := s.GetDiscoveryJob(ctx, jobA.ID, userA.ID)
		require.NoError(t, err)
		assert.Equal(t, model.DiscoveryStatusCancelled, fetchedAAfterCancel.Status)

		// Cancellation is idempotent
		err = s.CancelDiscoveryJob(ctx, jobA.ID, userA.ID)
		assert.NoError(t, err, "Cancelling an already cancelled job must be idempotent")

		// Running -> Cancelled is allowed
		jobRunning := &model.DiscoveryJob{
			UserID:   userA.ID,
			Lat:      12.9716,
			Lng:      77.5946,
			RadiusKM: 5.0,
			Status:   model.DiscoveryStatusRunning,
		}
		err = s.CreateDiscoveryJob(ctx, jobRunning)
		require.NoError(t, err)
		err = s.CancelDiscoveryJob(ctx, jobRunning.ID, userA.ID)
		require.NoError(t, err)

		// Completed -> Cancelled is NOT allowed
		jobCompleted := &model.DiscoveryJob{
			UserID:   userA.ID,
			Lat:      12.9716,
			Lng:      77.5946,
			RadiusKM: 5.0,
			Status:   model.DiscoveryStatusCompleted,
		}
		err = s.CreateDiscoveryJob(ctx, jobCompleted)
		require.NoError(t, err)
		err = s.CancelDiscoveryJob(ctx, jobCompleted.ID, userA.ID)
		assert.ErrorIs(t, err, ErrInvalidJobState, "Completed job cannot be cancelled")
	})
}

func TestDiscoveryStore_SourceRunUpsert(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		ctx := context.Background()

		user := &model.User{
			ID:       uuid.New(),
			Email:    fmt.Sprintf("user-%s@example.com", uuid.New().String()[:8]),
			Password: "hash",
		}
		require.NoError(t, s.CreateUser(ctx, user))

		job := &model.DiscoveryJob{
			UserID:   user.ID,
			Lat:      12.9716,
			Lng:      77.5946,
			RadiusKM: 10.0,
		}
		err := s.CreateDiscoveryJob(ctx, job)
		require.NoError(t, err)

		now := time.Now().Truncate(time.Millisecond)
		run := &model.DiscoverySourceRun{
			DiscoveryJobID: job.ID,
			Source:         "greenhouse",
			SourceFamily:   "job_ats",
			Status:         model.DiscoveryStatusRunning,
			Attempts:       1,
			StartedAt:      &now,
		}

		err = s.UpsertDiscoverySourceRun(ctx, run)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, run.ID)

		runs, err := s.GetDiscoverySourceRuns(ctx, job.ID)
		require.NoError(t, err)
		require.Len(t, runs, 1)
		assert.Equal(t, "greenhouse", runs[0].Source)
		assert.Equal(t, model.DiscoveryStatusRunning, runs[0].Status)

		// Update the existing source run (idempotent upsert by job_id + source)
		finished := now.Add(2 * time.Second)
		runUpdate := &model.DiscoverySourceRun{
			DiscoveryJobID: job.ID,
			Source:         "greenhouse",
			SourceFamily:   "job_ats",
			Status:         model.DiscoveryStatusCompleted,
			Attempts:       1,
			CompanyCount:   3,
			JobCount:       8,
			EvidenceCount:  12,
			DurationMS:     2000,
			StartedAt:      &now,
			FinishedAt:     &finished,
		}
		err = s.UpsertDiscoverySourceRun(ctx, runUpdate)
		require.NoError(t, err)

		runsAfterUpdate, err := s.GetDiscoverySourceRuns(ctx, job.ID)
		require.NoError(t, err)
		require.Len(t, runsAfterUpdate, 1, "Upserting existing source run should update in-place without duplicates")
		assert.Equal(t, model.DiscoveryStatusCompleted, runsAfterUpdate[0].Status)
		assert.Equal(t, 3, runsAfterUpdate[0].CompanyCount)
		assert.Equal(t, 8, runsAfterUpdate[0].JobCount)
		assert.Equal(t, int64(2000), runsAfterUpdate[0].DurationMS)

		// Verify GetDiscoveryJob embeds source runs
		fetchedJob, err := s.GetDiscoveryJob(ctx, job.ID, user.ID)
		require.NoError(t, err)
		assert.Len(t, fetchedJob.SourceRuns, 1)
	})
}

func TestTechnicalJobStore_RecentByCompany(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		ctx := context.Background()

		uid := uuid.New().String()[:8]
		companyID := uuid.New()
		company := &model.Company{
			ID:             companyID,
			Name:           fmt.Sprintf("Acme Technologies %s", uid),
			NormalizedName: fmt.Sprintf("acme technologies %s", uid),
		}
		err := s.CreateCompany(ctx, company)
		require.NoError(t, err)

		// Add location with PresenceType
		loc := &model.Location{
			CompanyID:    companyID,
			Address:      "MG Road, Bangalore",
			Lat:          12.9750,
			Lng:          77.6090,
			Confidence:   0.9,
			PresenceType: model.PresenceTypeConfirmedOffice,
			Verified:     true,
		}
		err = s.CreateLocation(ctx, loc)
		require.NoError(t, err)

		now := time.Now().Truncate(time.Millisecond)
		fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
		twentyDaysAgo := now.Add(-20 * 24 * time.Hour)
		twoDaysAgo := now.Add(-2 * 24 * time.Hour)
		twentyFiveDaysAgo := now.Add(-25 * 24 * time.Hour)

		// 1: Posted 5 days ago (recent)
		jobRecentPosted := &model.TechnicalJobPosting{
			CompanyID:               companyID,
			Source:                  "greenhouse",
			SourceFamily:            "job_ats",
			Title:                   "Backend Engineer",
			NormalizedTitle:         "backend engineer",
			ContentHash:             fmt.Sprintf("hash-recent-posted-%s", uid),
			WorkArrangement:         model.WorkArrangementInOffice,
			PublicationState:        model.PublicationStatePostedRecently,
			PostedAt:                &fiveDaysAgo,
			PostedAtConfidence:      1.0,
			FirstSeenAt:             fiveDaysAgo,
			LastSeenAt:              now,
			IsActive:                true,
			TechnicalClassification: "software_engineering",
			RuleVersion:             "v1",
		}
		err = s.UpsertTechnicalJob(ctx, jobRecentPosted)
		require.NoError(t, err)

		// 2: Posted 20 days ago (stale)
		jobStalePosted := &model.TechnicalJobPosting{
			CompanyID:               companyID,
			Source:                  "lever",
			SourceFamily:            "job_ats",
			Title:                   "Site Reliability Engineer",
			NormalizedTitle:         "site reliability engineer",
			ContentHash:             fmt.Sprintf("hash-stale-posted-%s", uid),
			WorkArrangement:         model.WorkArrangementHybrid,
			PublicationState:        model.PublicationStateStale,
			PostedAt:                &twentyDaysAgo,
			PostedAtConfidence:      1.0,
			FirstSeenAt:             twentyDaysAgo,
			LastSeenAt:              twentyDaysAgo,
			IsActive:                true,
			TechnicalClassification: "infrastructure",
			RuleVersion:             "v1",
		}
		err = s.UpsertTechnicalJob(ctx, jobStalePosted)
		require.NoError(t, err)

		// 3: No posted_at, but last_seen 2 days ago (observed recently)
		jobRecentObserved := &model.TechnicalJobPosting{
			CompanyID:               companyID,
			Source:                  "careers_site",
			SourceFamily:            "official_site",
			Title:                   "Data Engineer",
			NormalizedTitle:         "data engineer",
			ContentHash:             fmt.Sprintf("hash-recent-observed-%s", uid),
			WorkArrangement:         model.WorkArrangementRemote,
			PublicationState:        model.PublicationStateObservedRecently,
			FirstSeenAt:             twentyDaysAgo,
			LastSeenAt:              twoDaysAgo,
			IsActive:                true,
			TechnicalClassification: "data_engineering",
			RuleVersion:             "v1",
		}
		err = s.UpsertTechnicalJob(ctx, jobRecentObserved)
		require.NoError(t, err)

		// 4: No posted_at, last_seen 25 days ago (old observed)
		jobOldObserved := &model.TechnicalJobPosting{
			CompanyID:               companyID,
			Source:                  "careers_site_old",
			SourceFamily:            "official_site",
			Title:                   "Security Analyst",
			NormalizedTitle:         "security analyst",
			ContentHash:             fmt.Sprintf("hash-old-observed-%s", uid),
			WorkArrangement:         model.WorkArrangementUnknown,
			PublicationState:        model.PublicationStateStale,
			FirstSeenAt:             twentyFiveDaysAgo,
			LastSeenAt:              twentyFiveDaysAgo,
			IsActive:                true,
			TechnicalClassification: "security",
			RuleVersion:             "v1",
		}
		err = s.UpsertTechnicalJob(ctx, jobOldObserved)
		require.NoError(t, err)

		// Filter since 14 days ago: should return jobRecentPosted and jobRecentObserved
		fourteenDaysAgo := now.Add(-14 * 24 * time.Hour)
		recentJobs, err := s.GetTechnicalJobsByCompany(ctx, companyID, fourteenDaysAgo)
		require.NoError(t, err)
		require.Len(t, recentJobs, 2)

		titles := []string{recentJobs[0].Title, recentJobs[1].Title}
		assert.Contains(t, titles, "Backend Engineer")
		assert.Contains(t, titles, "Data Engineer")
		assert.NotContains(t, titles, "Site Reliability Engineer")
		assert.NotContains(t, titles, "Security Analyst")
	})
}

func TestDiscoveryStore_LocationEvidenceAndPresence(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		ctx := context.Background()
		uid := uuid.New().String()[:8]

		companyID := uuid.New()
		err := s.CreateCompany(ctx, &model.Company{
			ID:             companyID,
			Name:           fmt.Sprintf("Nova Tech %s", uid),
			NormalizedName: fmt.Sprintf("nova tech %s", uid),
		})
		require.NoError(t, err)

		locID := uuid.New()
		err = s.CreateLocation(ctx, &model.Location{
			ID:           locID,
			CompanyID:    companyID,
			Address:      "Bellandur, Bangalore",
			Lat:          12.9279,
			Lng:          77.6811,
			PresenceType: model.PresenceTypeProbableOffice,
			Confidence:   0.6,
			Verified:     false,
		})
		require.NoError(t, err)

		// 1. Test UpdateLocationPresence
		err = s.UpdateLocationPresence(ctx, locID, model.PresenceTypeConfirmedOffice, 0.9, true)
		require.NoError(t, err)

		locs, err := s.GetLocationsByCompany(ctx, companyID)
		require.NoError(t, err)
		require.Len(t, locs, 1)
		assert.Equal(t, model.PresenceTypeConfirmedOffice, locs[0].PresenceType)
		assert.InDelta(t, 0.9, locs[0].Confidence, 0.01)
		assert.True(t, locs[0].Verified)

		// 2. Add multiple sightings with same and different source families
		sighting1 := model.Sighting{
			ID:           uuid.New(),
			CompanyID:    &companyID,
			LocationID:   &locID,
			Source:       "official_site",
			SourceFamily: "official_site",
			CompanyName:  fmt.Sprintf("Nova Tech %s", uid),
			Lat:          12.9279,
			Lng:          77.6811,
		}
		sighting2 := model.Sighting{
			ID:           uuid.New(),
			CompanyID:    &companyID,
			LocationID:   &locID,
			Source:       "osm",
			SourceFamily: "open_dataset",
			CompanyName:  fmt.Sprintf("Nova Tech %s", uid),
			Lat:          12.9280, // ~15m away
			Lng:          77.6812,
		}
		sighting3 := model.Sighting{
			ID:           uuid.New(),
			CompanyID:    &companyID,
			LocationID:   &locID,
			Source:       "wikidata",
			SourceFamily: "open_dataset", // Duplicate family
			CompanyName:  fmt.Sprintf("Nova Tech %s", uid),
			Lat:          12.9281,
			Lng:          77.6813,
		}
		err = s.SaveSightings(ctx, "mixed", []model.Sighting{sighting1, sighting2, sighting3})
		require.NoError(t, err)

		// 3. Test GetLocationEvidenceSummaries returns distinct source families
		summaries, err := s.GetLocationEvidenceSummaries(ctx, companyID)
		require.NoError(t, err)
		require.Len(t, summaries, 1)

		summary := summaries[0]
		assert.Equal(t, locID, summary.LocationID)
		assert.Contains(t, []string(summary.SourceFamilies), "official_site")
		assert.Contains(t, []string(summary.SourceFamilies), "open_dataset")
		assert.Len(t, summary.SourceFamilies, 2)
		assert.Contains(t, []string(summary.EvidenceTypes), "company")
	})
}

