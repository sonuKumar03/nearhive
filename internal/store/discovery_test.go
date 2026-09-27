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

func TestDiscoveryIdempotency_StoreSightingAndJob(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		ctx := context.Background()
		uid := uuid.New().String()[:8]

		// Common company for technical jobs
		companyID := uuid.New()
		err := s.CreateCompany(ctx, &model.Company{
			ID:             companyID,
			Name:           fmt.Sprintf("Idemp Corp %s", uid),
			NormalizedName: fmt.Sprintf("idemp corp %s", uid),
		})
		require.NoError(t, err)

		t1 := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
		t2 := time.Now().Truncate(time.Second)

		// 1. Sighting with (source, source_record_id)
		src1 := fmt.Sprintf("src1_%s", uid)
		recID1 := "record-001"
		s1 := &model.Sighting{
			ID:             uuid.New(),
			Source:         src1,
			SourceFamily:   "official_site",
			SourceRecordID: &recID1,
			CompanyName:    "Acme Sighting",
			RawAddress:     "100 First St",
			Lat:            12.91,
			Lng:            77.61,
			FirstSeenAt:    t1,
			LastSeenAt:     t1,
			ScrapedAt:      t1,
		}
		err = s.UpsertDiscoverySighting(ctx, s1)
		require.NoError(t, err)

		s1Update := &model.Sighting{
			ID:             uuid.New(),
			Source:         src1,
			SourceFamily:   "official_site",
			SourceRecordID: &recID1,
			CompanyName:    "Acme Sighting",
			RawAddress:     "100 First St Updated",
			Lat:            12.91,
			Lng:            77.61,
			FirstSeenAt:    t1,
			LastSeenAt:     t2,
			ScrapedAt:      t2,
		}
		err = s.UpsertDiscoverySighting(ctx, s1Update)
		require.NoError(t, err)
		assert.Equal(t, s1.ID, s1Update.ID, "Sighting ID should be preserved across idempotency updates")

		// 2. Sighting with missing source_record_id and identical content_hash
		src2 := fmt.Sprintf("src2_%s", uid)
		hash2 := fmt.Sprintf("hash_%s", uid)
		s2 := &model.Sighting{
			ID:           uuid.New(),
			Source:       src2,
			SourceFamily: "public_directory",
			ContentHash:  &hash2,
			CompanyName:  "Beta Sighting",
			RawAddress:   "200 Second St",
			FirstSeenAt:  t1,
			LastSeenAt:   t1,
			ScrapedAt:    t1,
		}
		err = s.UpsertDiscoverySighting(ctx, s2)
		require.NoError(t, err)

		s2Update := &model.Sighting{
			ID:           uuid.New(),
			Source:       src2,
			SourceFamily: "public_directory",
			ContentHash:  &hash2,
			CompanyName:  "Beta Sighting",
			RawAddress:   "200 Second St Updated",
			FirstSeenAt:  t1,
			LastSeenAt:   t2,
			ScrapedAt:    t2,
		}
		err = s.UpsertDiscoverySighting(ctx, s2Update)
		require.NoError(t, err)
		assert.Equal(t, s2.ID, s2Update.ID)

		// 3. Technical Job with (source, source_job_id)
		srcJob1 := fmt.Sprintf("srcjob1_%s", uid)
		sJobID := "sjob-999"
		j1 := &model.TechnicalJobPosting{
			ID:                      uuid.New(),
			CompanyID:               companyID,
			Source:                  srcJob1,
			SourceFamily:            "job_ats",
			SourceJobID:             &sJobID,
			Title:                   "Staff Engineer",
			NormalizedTitle:         "staff engineer",
			ContentHash:             fmt.Sprintf("ch1_%s", uid),
			FirstSeenAt:             t1,
			LastSeenAt:              t1,
			IsActive:                true,
			TechnicalClassification: "software_engineering",
			RuleVersion:             "v1",
		}
		err = s.UpsertTechnicalJob(ctx, j1)
		require.NoError(t, err)

		j1Update := &model.TechnicalJobPosting{
			ID:                      uuid.New(),
			CompanyID:               companyID,
			Source:                  srcJob1,
			SourceFamily:            "job_ats",
			SourceJobID:             &sJobID,
			Title:                   "Staff Engineer (Updated)",
			NormalizedTitle:         "staff engineer (updated)",
			ContentHash:             fmt.Sprintf("ch1_diff_%s", uid),
			FirstSeenAt:             t1,
			LastSeenAt:              t2,
			IsActive:                true,
			TechnicalClassification: "software_engineering",
			RuleVersion:             "v1",
		}
		err = s.UpsertTechnicalJob(ctx, j1Update)
		require.NoError(t, err)

		// 4. Technical Job with missing source_job_id and identical content_hash
		srcJob2 := fmt.Sprintf("srcjob2_%s", uid)
		jHash2 := fmt.Sprintf("jhash2_%s", uid)
		j2 := &model.TechnicalJobPosting{
			ID:                      uuid.New(),
			CompanyID:               companyID,
			Source:                  srcJob2,
			SourceFamily:            "official_site",
			Title:                   "DevOps Engineer",
			NormalizedTitle:         "devops engineer",
			ContentHash:             jHash2,
			FirstSeenAt:             t1,
			LastSeenAt:              t1,
			IsActive:                true,
			TechnicalClassification: "infrastructure",
			RuleVersion:             "v1",
		}
		err = s.UpsertTechnicalJob(ctx, j2)
		require.NoError(t, err)

		j2Update := &model.TechnicalJobPosting{
			ID:                      uuid.New(),
			CompanyID:               companyID,
			Source:                  srcJob2,
			SourceFamily:            "official_site",
			Title:                   "DevOps Engineer (Updated)",
			NormalizedTitle:         "devops engineer (updated)",
			ContentHash:             jHash2,
			FirstSeenAt:             t1,
			LastSeenAt:              t2,
			IsActive:                true,
			TechnicalClassification: "infrastructure",
			RuleVersion:             "v1",
		}
		err = s.UpsertTechnicalJob(ctx, j2Update)
		require.NoError(t, err)

		// Verification: Query jobs for company
		jobs, err := s.GetTechnicalJobsByCompany(ctx, companyID, time.Time{})
		require.NoError(t, err)
		assert.Len(t, jobs, 2, "Should have exactly 2 distinct technical jobs (1 from j1, 1 from j2)")

		for _, j := range jobs {
			assert.WithinDuration(t, t2, j.LastSeenAt, 2*time.Second, "last_seen_at should be updated to t2")
		}
	})
}

func TestSearchDiscovery(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		ctx := context.Background()
		uid := uuid.New().String()[:8]

		// Center coordinates: Indiranagar, Bangalore
		centerLat := 12.9716
		centerLng := 77.6412

		// Company 1: Confirmed Office with 2 recent trustworthy technical jobs (in_office, remote),
		// 1 stale job (hybrid) and 1 observed_recently job (untrusted posted date)
		c1 := &model.Company{
			ID:             uuid.New(),
			Name:           fmt.Sprintf("Alpha Corp %s", uid),
			NormalizedName: fmt.Sprintf("alpha corp %s", uid),
		}
		require.NoError(t, s.CreateCompany(ctx, c1))
		loc1 := &model.Location{
			CompanyID:    c1.ID,
			Address:      "Indiranagar 100ft Rd",
			Lat:          centerLat + 0.001,
			Lng:          centerLng + 0.001,
			Confidence:   0.95,
			PresenceType: model.PresenceTypeConfirmedOffice,
			Verified:     true,
		}
		require.NoError(t, s.CreateLocation(ctx, loc1))

		now := time.Now().Truncate(time.Millisecond)
		fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
		tenDaysAgo := now.Add(-10 * 24 * time.Hour)
		twentyDaysAgo := now.Add(-20 * 24 * time.Hour)
		twoDaysAgo := now.Add(-2 * 24 * time.Hour)

		// Trustworthy recent job 1 (in_office)
		require.NoError(t, s.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
			CompanyID:               c1.ID,
			Source:                  fmt.Sprintf("gh-%s-1", uid),
			SourceFamily:            "job_ats",
			Title:                   "Backend Engineer",
			NormalizedTitle:         "backend engineer",
			ContentHash:             fmt.Sprintf("hash-alpha-1-%s", uid),
			WorkArrangement:         model.WorkArrangementInOffice,
			PublicationState:        model.PublicationStatePostedRecently,
			PostedAt:                &fiveDaysAgo,
			PostedAtConfidence:      1.0,
			FirstSeenAt:             fiveDaysAgo,
			LastSeenAt:              now,
			IsActive:                true,
			TechnicalClassification: "software_engineering",
			RuleVersion:             "v1",
		}))

		// Trustworthy recent job 2 (remote)
		require.NoError(t, s.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
			CompanyID:               c1.ID,
			Source:                  fmt.Sprintf("gh-%s-2", uid),
			SourceFamily:            "job_ats",
			Title:                   "Frontend Engineer",
			NormalizedTitle:         "frontend engineer",
			ContentHash:             fmt.Sprintf("hash-alpha-2-%s", uid),
			WorkArrangement:         model.WorkArrangementRemote,
			PublicationState:        model.PublicationStatePostedRecently,
			PostedAt:                &tenDaysAgo,
			PostedAtConfidence:      1.0,
			FirstSeenAt:             tenDaysAgo,
			LastSeenAt:              now,
			IsActive:                true,
			TechnicalClassification: "software_engineering",
			RuleVersion:             "v1",
		}))

		// Stale job (>14 days, posted 20 days ago) - should NOT count
		require.NoError(t, s.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
			CompanyID:               c1.ID,
			Source:                  fmt.Sprintf("gh-%s-3", uid),
			SourceFamily:            "job_ats",
			Title:                   "DevOps Engineer",
			NormalizedTitle:         "devops engineer",
			ContentHash:             fmt.Sprintf("hash-alpha-3-%s", uid),
			WorkArrangement:         model.WorkArrangementHybrid,
			PublicationState:        model.PublicationStateStale,
			PostedAt:                &twentyDaysAgo,
			PostedAtConfidence:      1.0,
			FirstSeenAt:             twentyDaysAgo,
			LastSeenAt:              twentyDaysAgo,
			IsActive:                true,
			TechnicalClassification: "infrastructure",
			RuleVersion:             "v1",
		}))

		// Untrustworthy date / observed recently job - should NOT count in recent_technical_job_count
		require.NoError(t, s.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
			CompanyID:               c1.ID,
			Source:                  fmt.Sprintf("careers-%s-4", uid),
			SourceFamily:            "official_site",
			Title:                   "Data Scientist",
			NormalizedTitle:         "data scientist",
			ContentHash:             fmt.Sprintf("hash-alpha-4-%s", uid),
			WorkArrangement:         model.WorkArrangementHybrid,
			PublicationState:        model.PublicationStateObservedRecently,
			PostedAtConfidence:      0.0,
			FirstSeenAt:             twoDaysAgo,
			LastSeenAt:              twoDaysAgo,
			IsActive:                true,
			TechnicalClassification: "data_engineering",
			RuleVersion:             "v1",
		}))

		// Company 2: Probable Office, no jobs
		c2 := &model.Company{
			ID:             uuid.New(),
			Name:           fmt.Sprintf("Beta Tech %s", uid),
			NormalizedName: fmt.Sprintf("beta tech %s", uid),
		}
		require.NoError(t, s.CreateCompany(ctx, c2))
		loc2 := &model.Location{
			CompanyID:    c2.ID,
			Address:      "Indiranagar 12th Main",
			Lat:          centerLat + 0.002,
			Lng:          centerLng + 0.002,
			Confidence:   0.7,
			PresenceType: model.PresenceTypeProbableOffice,
			Verified:     false,
		}
		require.NoError(t, s.CreateLocation(ctx, loc2))

		// Company 3: Job Location Only, with 1 hybrid job posted 3 days ago
		c3 := &model.Company{
			ID:             uuid.New(),
			Name:           fmt.Sprintf("Gamma Labs %s", uid),
			NormalizedName: fmt.Sprintf("gamma labs %s", uid),
		}
		require.NoError(t, s.CreateCompany(ctx, c3))
		loc3 := &model.Location{
			CompanyID:    c3.ID,
			Address:      "Indiranagar Double Rd",
			Lat:          centerLat + 0.003,
			Lng:          centerLng + 0.003,
			Confidence:   0.5,
			PresenceType: model.PresenceTypeJobLocationOnly,
			Verified:     false,
		}
		require.NoError(t, s.CreateLocation(ctx, loc3))
		threeDaysAgo := now.Add(-3 * 24 * time.Hour)
		require.NoError(t, s.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
			CompanyID:               c3.ID,
			Source:                  fmt.Sprintf("lever-%s-1", uid),
			SourceFamily:            "job_ats",
			Title:                   "Security Analyst",
			NormalizedTitle:         "security analyst",
			ContentHash:             fmt.Sprintf("hash-gamma-1-%s", uid),
			WorkArrangement:         model.WorkArrangementHybrid,
			PublicationState:        model.PublicationStatePostedRecently,
			PostedAt:                &threeDaysAgo,
			PostedAtConfidence:      1.0,
			FirstSeenAt:             threeDaysAgo,
			LastSeenAt:              now,
			IsActive:                true,
			TechnicalClassification: "security",
			RuleVersion:             "v1",
		}))

		// Company 4: Remote-only jobs, NO location within radius (e.g., location 500km away)
		c4 := &model.Company{
			ID:             uuid.New(),
			Name:           fmt.Sprintf("RemoteOnly Corp %s", uid),
			NormalizedName: fmt.Sprintf("remoteonly corp %s", uid),
		}
		require.NoError(t, s.CreateCompany(ctx, c4))
		require.NoError(t, s.UpsertTechnicalJob(ctx, &model.TechnicalJobPosting{
			CompanyID:               c4.ID,
			Source:                  fmt.Sprintf("remote-%s-1", uid),
			SourceFamily:            "job_ats",
			Title:                   "Staff Engineer (Remote)",
			NormalizedTitle:         "staff engineer remote",
			ContentHash:             fmt.Sprintf("hash-remote-1-%s", uid),
			WorkArrangement:         model.WorkArrangementRemote,
			PublicationState:        model.PublicationStatePostedRecently,
			PostedAt:                &fiveDaysAgo,
			PostedAtConfidence:      1.0,
			FirstSeenAt:             fiveDaysAgo,
			LastSeenAt:              now,
			IsActive:                true,
			TechnicalClassification: "software_engineering",
			RuleVersion:             "v1",
		}))
		loc4 := &model.Location{
			CompanyID:    c4.ID,
			Address:      "Chennai, Tamil Nadu",
			Lat:          13.0827,
			Lng:          80.2707,
			Confidence:   0.8,
			PresenceType: model.PresenceTypeConfirmedOffice,
		}
		require.NoError(t, s.CreateLocation(ctx, loc4))

		// Execute Search around Indiranagar center with 5km radius
		results, err := s.Search(ctx, centerLat, centerLng, 5000, SearchOpts{})
		require.NoError(t, err)

		// c4 should NOT be in results
		for _, r := range results {
			assert.NotEqual(t, c4.ID, r.CompanyID, "Remote-only company without location in radius must not be returned")
		}

		// Find results for c1, c2, c3
		var res1, res2, res3 *model.CompanySearchResult
		for i := range results {
			if results[i].CompanyID == c1.ID {
				res1 = &results[i]
			} else if results[i].CompanyID == c2.ID {
				res2 = &results[i]
			} else if results[i].CompanyID == c3.ID {
				res3 = &results[i]
			}
		}

		// Verify c1
		require.NotNil(t, res1, "Alpha Corp should be found in search")
		assert.Equal(t, model.PresenceTypeConfirmedOffice, res1.PresenceType)
		assert.Equal(t, 2, res1.RecentTechnicalJobCount, "Should count only 2 trustworthy jobs within 14 days")
		assert.ElementsMatch(t, []model.WorkArrangement{model.WorkArrangementInOffice, model.WorkArrangementRemote}, res1.Arrangements)

		// Verify c2
		require.NotNil(t, res2, "Beta Tech should be found in search")
		assert.Equal(t, model.PresenceTypeProbableOffice, res2.PresenceType)
		assert.Equal(t, 0, res2.RecentTechnicalJobCount)
		assert.Empty(t, res2.Arrangements)

		// Verify c3
		require.NotNil(t, res3, "Gamma Labs should be found in search")
		assert.Equal(t, model.PresenceTypeJobLocationOnly, res3.PresenceType)
		assert.Equal(t, 1, res3.RecentTechnicalJobCount)
		assert.ElementsMatch(t, []model.WorkArrangement{model.WorkArrangementHybrid}, res3.Arrangements)
	})
}


