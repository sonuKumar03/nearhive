export interface Company {
  id: string;
  name: string;
  normalized_name: string;
  domain?: string;
  industry?: string;
  employee_count?: string;
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
