export type DiscoveryStatus = 'pending' | 'running' | 'completed' | 'partial' | 'failed' | 'cancelled';
export type PresenceType = 'confirmed_office' | 'probable_office' | 'job_location_only';
export type WorkArrangement = 'in_office' | 'hybrid' | 'remote' | 'unknown';
export type PublicationState = 'posted_recently' | 'observed_recently' | 'stale';

export interface Company {
  id: string;
  name: string;
  normalized_name: string;
  domain?: string;
  industry?: string;
  employee_count?: string;
  presence_type?: PresenceType;
  recent_technical_job_count?: number;
  arrangements?: WorkArrangement[];
  created_at: string;
  updated_at: string;
}

export interface Location {
  id: string;
  company_id: string;
  address: string;
  lat: number;
  lng: number;
  confidence: number;
  presence_type?: PresenceType;
  label?: string;
  city?: string;
  state?: string;
  country?: string;
  verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface CompanyDetailResponse {
  company: Company;
  locations: Location[];
}

export interface CompanySearchResult {
  id: string;
  name: string;
  domain?: string;
  industry?: string;
  employee_count?: string;
  location_id: string;
  label?: string;
  address: string;
  city?: string;
  lat: number;
  lng: number;
  confidence: number;
  distance_meters: number;
  presence_type?: PresenceType;
  recent_technical_job_count?: number;
  arrangements?: WorkArrangement[];
  verified: boolean;
}

export interface SpatialCluster {
  cluster_id: number;
  count: number;
  lat: number;
  lng: number;
}

export interface Sighting {
  id: string;
  source: string;
  source_url?: string;
  company_name: string;
  raw_address: string;
  lat: number;
  lng: number;
  metadata?: Record<string, any>;
  company_id?: string;
  location_id?: string;
  scraped_at: string;
}

export interface ScrapeTask {
  id: string;
  job_id: string;
  source: string;
  status: 'pending' | 'running' | 'done' | 'failed' | 'cancelled';
  sightings: number;
  error?: string;
  duration_ms: number;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}

export interface ScrapeJob {
  id: string;
  source: string;
  status: 'pending' | 'running' | 'done' | 'failed' | 'cancelled';
  region?: string;
  lat?: number;
  lng?: number;
  radius_km?: number;
  worker_id?: string;
  sightings: number;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  tasks?: ScrapeTask[];
}

export interface User {
  id: string;
  email: string;
  created_at?: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface SearchParams {
  lat: number;
  lng: number;
  radius_km: number;
  min_confidence?: number;
  industry?: string;
  q?: string;
  limit?: number;
  page?: number;
}

export interface ClusterParams {
  lat: number;
  lng: number;
  radius_km: number;
  k?: number;
}

export interface DiscoverySourceRun {
  id: string;
  discovery_job_id: string;
  source: string;
  source_family: string;
  status: DiscoveryStatus;
  attempts: number;
  company_count: number;
  job_count: number;
  evidence_count: number;
  error?: string;
  duration_ms: number;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at: string;
}

export interface DiscoveryJob {
  id: string;
  user_id: string;
  status: DiscoveryStatus;
  lat: number;
  lng: number;
  radius_km: number;
  worker_id?: string;
  lease_expires_at?: string;
  last_heartbeat_at?: string;
  attempts: number;
  max_attempts: number;
  error?: string;
  company_count: number;
  job_count: number;
  evidence_count: number;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at: string;
  source_runs?: DiscoverySourceRun[];
}

export interface TechnicalJobPosting {
  id: string;
  company_id: string;
  location_id?: string;
  discovery_job_id?: string;
  source: string;
  source_family: string;
  source_job_id?: string;
  canonical_url?: string;
  title: string;
  normalized_title: string;
  description_excerpt?: string;
  content_hash: string;
  location_raw?: string;
  lat?: number;
  lng?: number;
  work_arrangement: WorkArrangement;
  publication_state: PublicationState;
  posted_at?: string;
  posted_at_confidence: number;
  first_seen_at: string;
  last_seen_at: string;
  is_active: boolean;
  technical_classification: string;
  rule_version: string;
  classification_reasons?: string[];
  metadata?: Record<string, any>;
  created_at: string;
  updated_at: string;
}

export interface CreateDiscoveryJobParams {
  lat: number;
  lng: number;
  radius_km: number;
}

export interface CompanyTechnicalJobsResponse {
  jobs: TechnicalJobPosting[];
  technical_jobs: TechnicalJobPosting[];
}

export interface TechnicalJobSearchResult {
  id: string;
  company_id: string;
  company_name: string;
  company_domain?: string;
  title: string;
  normalized_title: string;
  description_excerpt?: string;
  canonical_url?: string;
  source: string;
  source_family: string;
  location_raw?: string;
  lat: number;
  lng: number;
  distance_meters: number;
  work_arrangement: WorkArrangement;
  publication_state: PublicationState;
  posted_at?: string;
  posted_at_confidence: number;
  first_seen_at: string;
  last_seen_at: string;
  metadata?: Record<string, any>;
}

export interface TechnicalJobSearchResponse {
  meta: {
    total: number;
    page: number;
    limit: number;
    radius_km: number;
    center: {
      lat: number;
      lng: number;
    };
  };
  jobs: TechnicalJobSearchResult[];
}

export interface NearbyJobsParams {
  lat: number;
  lng: number;
  radius_km?: number;
  q?: string;
  work_arrangement?: WorkArrangement;
  limit?: number;
  page?: number;
}
