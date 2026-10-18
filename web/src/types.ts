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

// Edge Rules types
export type RuleType = 'strip_query_params' | 'strip_cookies' | 'override_ttl' | 'add_surrogate_keys' | 'bypass_cache';

export interface EdgeRule {
  id: string;
  name: string;
  type: RuleType;
  enabled: boolean;
  pathPattern: string;
  stripParams?: string[];
  stripAll?: boolean;
  ttlSeconds?: number;
  surrogateKey?: string;
  statusCodes?: number[];
}

// Multi-CDN Comparator types
export interface FlavorComparison {
  flavor: CDNFlavor;
  flavorName: string;
  statusHeader: string;
  statusValue: string;
  cacheOutcome: CacheStatus;
  effectiveTtlSeconds: number;
  surrogatePrecedence: string;
  cookieHandling: string;
  queryStringBehavior: string;
  injectedHeaders: Record<string, string>;
  strippedHeaders: string[];
  keyNotes: string[];
}

export interface DiffItem {
  field: string;
  description: string;
  values: Record<string, string>;
  impact: string;
}

export interface ComparisonReport {
  url: string;
  method: string;
  statusCode: number;
  flavors: Record<string, FlavorComparison>;
  differences: DiffItem[];
  recommendations: string[];
  evaluatedAt: string;
}

// Vary Matrix types
export type RiskLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';

export interface VaryDimension {
  headerName: string;
  typicalCardinality: number;
  risk: RiskLevel;
  description: string;
  edgeRemediation: string;
  sampleValues: string[];
}

export interface SampleVariant {
  headers: Record<string, string>;
  cacheKeySuffix: string;
  collapsesWithFix: boolean;
}

export interface VaryAnalysisReport {
  rawVaryHeader: string;
  dimensions: VaryDimension[];
  estimatedPermutations: number;
  originPenetrationRisk: RiskLevel;
  estimatedMemoryMultiplier: number;
  normalizedVaryProposal: string;
  recommendations: string[];
  sampleVariants: SampleVariant[];
}

// CDN Config Export types
export interface GeneratedConfigs {
  nginxConf: string;
  fastlyVcl: string;
  cloudflareTerraform: string;
  cloudFrontTerraform: string;
}

// CI/CD Assertion types
export interface CheckDetail {
  name: string;
  expected: string;
  actual: string;
  passed: boolean;
  errorMessage?: string;
}

export interface AssertionResult {
  url: string;
  passed: boolean;
  httpStatus: number;
  cacheOutcome: string;
  calculatedTtl: number;
  auditGrade: string;
  auditScore: number;
  checks: CheckDetail[];
  durationMs: number;
  summary: string;
}

// cURL & HAR types
export interface CurlParsedRequest {
  method: string;
  url: string;
  headers: Record<string, string>;
  body?: string;
}

export interface HAREntryAnalysis {
  url: string;
  method: string;
  statusCode: number;
  mimeType: string;
  cacheEligible: boolean;
  calculatedTtl: number;
  cacheControl: string;
  vary: string;
  surrogateTags: string[];
  auditGrade: string;
  auditScore: number;
  blockerReason?: string;
  detectedCdnStatus?: string;
}

export interface HARReport {
  totalRequests: number;
  cacheableRequests: number;
  bypassRequests: number;
  cacheabilityRatio: number;
  averageTtlSeconds: number;
  topBlockers: Record<string, number>;
  entries: HAREntryAnalysis[];
}
