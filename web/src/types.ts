export type CacheStatus = 'HIT' | 'MISS' | 'EXPIRED' | 'STALE' | 'BYPASS' | 'REVALIDATED' | 'DYNAMIC';

export type CDNFlavor = 'cloudflare' | 'fastly' | 'cloudfront' | 'vercel' | 'rfc9111';

export interface DecisionPhase {
  name: string;
  passed: boolean;
  description: string;
  details: string[];
}

export interface Recommendation {
  level: 'critical' | 'warning' | 'info';
  title: string;
  message: string;
}

export interface Explanation {
  id: string;
  timestamp: string;
  url: string;
  method: string;
  finalStatus: CacheStatus;
  statusCode: number;
  latencyMs: number;
  originLatencyMs: number;
  cdnFlavor: CDNFlavor;
  primaryKey: string;
  varyHash?: string;
  varyHeaders?: string[];
  tags?: string[];
  currentAgeSeconds: number;
  freshnessTtlSeconds: number;
  phases: DecisionPhase[];
  recommendations: Recommendation[];
  requestHeaders: Record<string, string[]>;
  responseHeaders: Record<string, string[]>;
}

export interface EntrySummary {
  key: string;
  method: string;
  url: string;
  statusCode: number;
  bodySize: number;
  tags: string[];
  storedAt: string;
  freshnessLifetimeSeconds: number;
  currentAgeSeconds: number;
  isFresh: boolean;
  isInSWR: boolean;
  isSoftPurged: boolean;
  hits: number;
  etag?: string;
  varyHeaders?: Record<string, string>;
}

export interface CacheStats {
  totalEntries: number;
  totalHits: number;
  totalMisses: number;
  totalBypasses: number;
  totalBytes: number;
  hitRatio: number;
}

export interface TagSummary {
  tag: string;
  entriesCount: number;
  sampleKeys: string[];
  lastSeen: string;
}

export interface PurgeResult {
  id: string;
  type: string;
  target: string;
  isSoft: boolean;
  affectedCount: number;
  affectedKeys: string[];
  timestamp: string;
  durationMs: number;
}

export interface RequestSample {
  index: number;
  durationMs: number;
  statusCode: number;
  cacheStatus: string;
  wasOrigin: boolean;
  error?: string;
}

export interface LatencyPercentiles {
  minMs: number;
  maxMs: number;
  avgMs: number;
  p50Ms: number;
  p90Ms: number;
  p95Ms: number;
  p99Ms: number;
}

export interface StampedeReport {
  targetUrl: string;
  concurrency: number;
  coalescingEnabled: boolean;
  totalDurationMs: number;
  originHits: number;
  edgeHits: number;
  penetrationRate: number;
  percentiles: LatencyPercentiles;
  samples: RequestSample[];
  resilienceScore: number;
  recommendationNotes: string;
}

export interface MockRoute {
  id: string;
  path: string;
  method: string;
  statusCode: number;
  headers: Record<string, string>;
  body: string;
  delayMs: number;
  description: string;
  hitCount: number;
}

export interface AuditFinding {
  severity: 'critical' | 'high' | 'medium' | 'low' | 'good';
  title: string;
  description: string;
  ruleId: string;
  suggestion: string;
}

export interface AuditReport {
  score: number;
  grade: string;
  findings: AuditFinding[];
  cacheControl: string;
  vary: string;
  etag: string;
  lastModified: string;
  surrogateKey: string;
}

export interface CDNConfig {
  activeFlavor: CDNFlavor;
  stripSurrogateKeys: boolean;
  stripSetCookieOnHit: boolean;
  enableStaleIfError: boolean;
}
