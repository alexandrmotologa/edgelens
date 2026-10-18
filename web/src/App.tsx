import React, { useState, useEffect } from 'react';
import {
  Activity,
  Layers,
  Zap,
  Shield,
  Server,
  FileCode,
  RefreshCw,
  Trash2,
  Play,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Info,
  ExternalLink,
  ChevronRight,
  Database,
  ArrowRight,
  Flame,
  Globe,
  Radio,
  Sliders,
  Sparkles
} from 'lucide-react';
import {
  Explanation,
  EntrySummary,
  CacheStats,
  TagSummary,
  PurgeResult,
  StampedeReport,
  MockRoute,
  AuditReport,
  CDNFlavor,
  CDNConfig
} from './types';

export function App() {
  const [activeTab, setActiveTab] = useState<'traffic' | 'surrogate' | 'stampede' | 'cdn' | 'mock' | 'audit'>('traffic');
  const [stats, setStats] = useState<CacheStats | null>(null);
  const [cdnConfig, setCdnConfig] = useState<CDNConfig | null>(null);
  const [explanations, setExplanations] = useState<Explanation[]>([]);
  const [selectedExp, setSelectedExp] = useState<Explanation | null>(null);
  const [entries, setEntries] = useState<EntrySummary[]>([]);
  const [tags, setTags] = useState<TagSummary[]>([]);
  const [purgeHistory, setPurgeHistory] = useState<PurgeResult[]>([]);
  const [mockRoutes, setMockRoutes] = useState<MockRoute[]>([]);
  const [testUrl, setTestUrl] = useState('/mock/scenarios/stale-while-revalidate');
  const [loadingRequest, setLoadingRequest] = useState(false);
  const [statusFilter, setStatusFilter] = useState<string>('ALL');
  const [sseConnected, setSseConnected] = useState(false);

  // Surrogate Key state
  const [purgeTagInput, setPurgeTagInput] = useState('');
  const [isSoftPurge, setIsSoftPurge] = useState(false);

  // Stampede state
  const [stampedeUrl, setStampedeUrl] = useState('/mock/scenarios/flash-sale');
  const [concurrency, setConcurrency] = useState(100);
  const [coalesceEnabled, setCoalesceEnabled] = useState(true);
  const [runningStampede, setRunningStampede] = useState(false);
  const [stampedeReport, setStampedeReport] = useState<StampedeReport | null>(null);

  // Audit state
  const [auditUrl, setAuditUrl] = useState('');
  const [customHeaders, setCustomHeaders] = useState('Cache-Control: public, max-age=3600\nVary: Accept-Encoding\nETag: "v1.0"');
  const [auditReport, setAuditReport] = useState<AuditReport | null>(null);
  const [auditing, setAuditing] = useState(false);

  // Fetch initial telemetry
  const fetchStats = async () => {
    try {
      const res = await fetch('/api/cache/stats');
      if (res.ok) setStats(await res.json());
    } catch (_) {}
  };

  const fetchTraffic = async () => {
    try {
      const res = await fetch('/api/traffic');
      if (res.ok) {
        const data: Explanation[] = await res.json();
        setExplanations(data);
        if (data.length > 0 && !selectedExp) {
          setSelectedExp(data[0]);
        }
      }
    } catch (_) {}
  };

  const fetchEntries = async () => {
    try {
      const res = await fetch('/api/cache/entries');
      if (res.ok) setEntries(await res.json());
    } catch (_) {}
  };

  const fetchTags = async () => {
    try {
      const res = await fetch('/api/cache/tags');
      if (res.ok) setTags(await res.json());
      const hRes = await fetch('/api/cache/history');
      if (hRes.ok) setPurgeHistory(await hRes.json());
    } catch (_) {}
  };

  const fetchCDNConfig = async () => {
    try {
      const res = await fetch('/api/cdn/config');
      if (res.ok) {
        const data = await res.json();
        setCdnConfig(data.config);
      }
    } catch (_) {}
  };

  const fetchMockRoutes = async () => {
    try {
      const res = await fetch('/api/mock/routes');
      if (res.ok) setMockRoutes(await res.json());
    } catch (_) {}
  };

  useEffect(() => {
    fetchStats();
    fetchTraffic();
    fetchEntries();
    fetchTags();
    fetchCDNConfig();
    fetchMockRoutes();

    // Setup SSE real-time stream
    const eventSource = new EventSource('/api/stream');
    eventSource.onopen = () => setSseConnected(true);
    eventSource.onerror = () => setSseConnected(false);

    eventSource.addEventListener('traffic', (e) => {
      try {
        const exp: Explanation = JSON.parse(e.data);
        setExplanations((prev) => [exp, ...prev.slice(0, 99)]);
        setSelectedExp((prev) => (prev ? prev : exp));
        fetchStats();
        fetchEntries();
      } catch (_) {}
    });

    eventSource.addEventListener('purge', () => {
      fetchTags();
      fetchEntries();
      fetchStats();
    });

    const interval = setInterval(() => {
      fetchStats();
      fetchEntries();
      fetchMockRoutes();
    }, 4000);

    return () => {
      eventSource.close();
      clearInterval(interval);
    };
  }, []);

  const triggerTestRequest = async (targetUrl = testUrl) => {
    setLoadingRequest(true);
    try {
      const fullProxyUrl = `/proxy?url=${encodeURIComponent(targetUrl)}`;
      const res = await fetch(fullProxyUrl, {
        headers: {
          'X-Requested-By': 'EdgeLens-Workbench',
        },
      });
      await res.text();
      await fetchTraffic();
      await fetchStats();
      await fetchEntries();
      await fetchTags();
    } catch (err) {
      console.error(err);
    } finally {
      setLoadingRequest(false);
    }
  };

  const executePurge = async (type: string, target: string, soft = isSoftPurge) => {
    try {
      const res = await fetch('/api/cache/purge', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type, target, isSoft: soft }),
      });
      if (res.ok) {
        await fetchTags();
        await fetchEntries();
        await fetchStats();
        setPurgeTagInput('');
      }
    } catch (err) {
      console.error(err);
    }
  };

  const switchCDNFlavor = async (flavor: CDNFlavor) => {
    try {
      const res = await fetch('/api/cdn/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ flavor }),
      });
      if (res.ok) {
        await fetchCDNConfig();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const runStampedeTest = async () => {
    setRunningStampede(true);
    setStampedeReport(null);
    try {
      const proxyTarget = `${window.location.origin}/proxy?url=${encodeURIComponent(stampedeUrl)}`;
      const res = await fetch('/api/stampede/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          targetUrl: proxyTarget,
          concurrency,
          enableCoalescing: coalesceEnabled,
        }),
      });
      if (res.ok) {
        setStampedeReport(await res.json());
      }
    } catch (err) {
      console.error(err);
    } finally {
      setRunningStampede(false);
      fetchStats();
      fetchMockRoutes();
    }
  };

  const runAudit = async () => {
    setAuditing(true);
    try {
      const headersMap: Record<string, string[]> = {};
      customHeaders.split('\n').forEach((line) => {
        const idx = line.indexOf(':');
        if (idx !== -1) {
          const key = line.slice(0, idx).trim();
          const val = line.slice(idx + 1).trim();
          if (key && val) {
            headersMap[key] = [val];
          }
        }
      });

      const res = await fetch('/api/audit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: auditUrl || undefined,
          headers: Object.keys(headersMap).length > 0 ? headersMap : undefined,
        }),
      });
      if (res.ok) {
        setAuditReport(await res.json());
      }
    } catch (err) {
      console.error(err);
    } finally {
      setAuditing(false);
    }
  };

  const filteredExplanations = explanations.filter((exp) => {
    if (statusFilter === 'ALL') return true;
    return exp.finalStatus === statusFilter;
  });

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'HIT':
        return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30';
      case 'MISS':
        return 'bg-amber-500/10 text-amber-400 border-amber-500/30';
      case 'STALE':
        return 'bg-cyan-500/10 text-cyan-400 border-cyan-500/30';
      case 'BYPASS':
      case 'DYNAMIC':
        return 'bg-purple-500/10 text-purple-400 border-purple-500/30';
      case 'REVALIDATED':
        return 'bg-blue-500/10 text-blue-400 border-blue-500/30';
      case 'EXPIRED':
        return 'bg-rose-500/10 text-rose-400 border-rose-500/30';
      default:
        return 'bg-slate-500/10 text-slate-400 border-slate-500/30';
    }
  };

  return (
    <div className="min-h-screen bg-[#080c14] text-[#e2e8f0] flex flex-col font-sans">
      {/* Top Navigation Bar */}
      <header className="border-b border-slate-800 bg-[#0b101b]/80 backdrop-blur sticky top-0 z-50 px-6 py-3 flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <div className="flex items-center space-x-2.5">
            <div className="w-8 h-8 rounded-lg bg-sky-500/20 border border-sky-400/40 flex items-center justify-center text-sky-400 font-bold shadow-lg shadow-sky-500/10">
              <Zap className="w-4 h-4" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <span className="font-bold text-base tracking-tight text-white">EdgeLens</span>
                <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-sky-950 text-sky-400 border border-sky-800/60">
                  RFC 9111
                </span>
              </div>
              <p className="text-xs text-slate-400">CDN Edge &amp; Surrogate Key Workbench</p>
            </div>
          </div>

          {/* Quick Telemetry Pills */}
          <div className="hidden lg:flex items-center space-x-2 text-xs border-l border-slate-800 pl-4 ml-2">
            <div className="px-2.5 py-1 rounded-md bg-slate-900 border border-slate-800 flex items-center space-x-2">
              <span className="text-slate-400">Hit Ratio:</span>
              <span className="font-mono font-semibold text-emerald-400">
                {stats ? stats.hitRatio.toFixed(1) : '0.0'}%
              </span>
            </div>
            <div className="px-2.5 py-1 rounded-md bg-slate-900 border border-slate-800 flex items-center space-x-2">
              <span className="text-slate-400">Cached:</span>
              <span className="font-mono font-semibold text-sky-400">{entries.length} items</span>
            </div>
            <div className="px-2.5 py-1 rounded-md bg-slate-900 border border-slate-800 flex items-center space-x-2">
              <span className="text-slate-400">CDN:</span>
              <span className="font-mono uppercase font-semibold text-amber-400">
                {cdnConfig?.activeFlavor || 'cloudflare'}
              </span>
            </div>
          </div>
        </div>

        {/* Workspaces Tabs */}
        <nav className="flex items-center space-x-1 bg-slate-900/80 p-1 rounded-lg border border-slate-800 text-xs">
          <button
            onClick={() => setActiveTab('traffic')}
            className={`px-3 py-1.5 rounded-md font-medium transition-colors flex items-center space-x-1.5 ${
              activeTab === 'traffic'
                ? 'bg-sky-500 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Activity className="w-3.5 h-3.5" />
            <span>Traffic &amp; Explainer</span>
          </button>
          <button
            onClick={() => setActiveTab('surrogate')}
            className={`px-3 py-1.5 rounded-md font-medium transition-colors flex items-center space-x-1.5 ${
              activeTab === 'surrogate'
                ? 'bg-sky-500 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Layers className="w-3.5 h-3.5" />
            <span>Surrogate Keys ({tags.length})</span>
          </button>
          <button
            onClick={() => setActiveTab('stampede')}
            className={`px-3 py-1.5 rounded-md font-medium transition-colors flex items-center space-x-1.5 ${
              activeTab === 'stampede'
                ? 'bg-sky-500 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Flame className="w-3.5 h-3.5 text-amber-400" />
            <span>Stampede Lab</span>
          </button>
          <button
            onClick={() => setActiveTab('cdn')}
            className={`px-3 py-1.5 rounded-md font-medium transition-colors flex items-center space-x-1.5 ${
              activeTab === 'cdn'
                ? 'bg-sky-500 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Globe className="w-3.5 h-3.5" />
            <span>CDN Flavors</span>
          </button>
          <button
            onClick={() => setActiveTab('mock')}
            className={`px-3 py-1.5 rounded-md font-medium transition-colors flex items-center space-x-1.5 ${
              activeTab === 'mock'
                ? 'bg-sky-500 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Server className="w-3.5 h-3.5" />
            <span>Mock Origin</span>
          </button>
          <button
            onClick={() => setActiveTab('audit')}
            className={`px-3 py-1.5 rounded-md font-medium transition-colors flex items-center space-x-1.5 ${
              activeTab === 'audit'
                ? 'bg-sky-500 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Shield className="w-3.5 h-3.5" />
            <span>Header Linter</span>
          </button>
        </nav>

        {/* Live SSE Indicator */}
        <div className="flex items-center space-x-2 text-xs">
          <span
            className={`w-2 h-2 rounded-full ${
              sseConnected ? 'bg-emerald-400 animate-pulse' : 'bg-rose-500'
            }`}
          />
          <span className="text-slate-400 font-mono text-[11px]">
            {sseConnected ? 'STREAM ACTIVE' : 'RECONNECTING'}
          </span>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col p-6 overflow-hidden max-w-7xl mx-auto w-full">
        {/* Quick Test URL Action Bar */}
        <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-3 mb-6 flex items-center space-x-3 shadow-md">
          <div className="flex items-center space-x-2 text-xs font-semibold uppercase tracking-wider text-slate-400 pl-2">
            <Radio className="w-4 h-4 text-sky-400" />
            <span>Send Request</span>
          </div>
          <div className="flex-1 relative">
            <input
              type="text"
              value={testUrl}
              onChange={(e) => setTestUrl(e.target.value)}
              placeholder="e.g. /mock/scenarios/stale-while-revalidate or external URL"
              className="w-full bg-slate-900 border border-slate-700/80 rounded-lg px-3.5 py-2 text-xs font-mono text-white placeholder-slate-500 focus:outline-none focus:border-sky-500"
            />
          </div>
          <button
            onClick={() => triggerTestRequest(testUrl)}
            disabled={loadingRequest}
            className="bg-sky-500 hover:bg-sky-400 text-white text-xs font-medium px-4 py-2 rounded-lg flex items-center space-x-2 transition shadow disabled:opacity-50"
          >
            {loadingRequest ? (
              <RefreshCw className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Play className="w-3.5 h-3.5 fill-current" />
            )}
            <span>Execute</span>
          </button>
        </div>

        {/* TAB 1: TRAFFIC & DECISION EXPLAINER */}
        {activeTab === 'traffic' && (
          <div className="flex-1 grid grid-cols-12 gap-6 min-h-0">
            {/* Left Column: Live Traffic Table */}
            <div className="col-span-5 bg-[#0b101b] border border-slate-800 rounded-xl flex flex-col overflow-hidden shadow-lg">
              <div className="p-3.5 border-b border-slate-800 flex items-center justify-between bg-slate-900/40">
                <div className="flex items-center space-x-2">
                  <Activity className="w-4 h-4 text-sky-400" />
                  <span className="text-xs font-semibold text-white">Live Edge Stream</span>
                  <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                    {filteredExplanations.length}
                  </span>
                </div>
                {/* Filter dropdown */}
                <select
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                  className="bg-slate-900 border border-slate-700 text-slate-300 text-[11px] rounded px-2 py-1 focus:outline-none"
                >
                  <option value="ALL">All Statuses</option>
                  <option value="HIT">HIT</option>
                  <option value="MISS">MISS</option>
                  <option value="STALE">STALE</option>
                  <option value="BYPASS">BYPASS</option>
                  <option value="REVALIDATED">REVALIDATED</option>
                  <option value="EXPIRED">EXPIRED</option>
                </select>
              </div>

              <div className="flex-1 overflow-y-auto divide-y divide-slate-800/60">
                {filteredExplanations.length === 0 ? (
                  <div className="p-8 text-center text-slate-500 text-xs">
                    No requests captured yet. Send a request above to observe edge decisions.
                  </div>
                ) : (
                  filteredExplanations.map((exp) => (
                    <div
                      key={exp.id}
                      onClick={() => setSelectedExp(exp)}
                      className={`p-3 cursor-pointer transition-colors text-xs flex items-center justify-between ${
                        selectedExp?.id === exp.id
                          ? 'bg-sky-500/10 border-l-2 border-sky-400'
                          : 'hover:bg-slate-900/60'
                      }`}
                    >
                      <div className="flex items-center space-x-2.5 min-w-0">
                        <span
                          className={`text-[10px] font-mono font-bold px-2 py-0.5 rounded border uppercase ${getStatusBadge(
                            exp.finalStatus
                          )}`}
                        >
                          {exp.finalStatus}
                        </span>
                        <div className="truncate">
                          <p className="font-mono text-slate-200 truncate">{exp.url}</p>
                          <p className="text-[10px] text-slate-400">
                            {new Date(exp.timestamp).toLocaleTimeString()} &bull; {exp.latencyMs}ms (origin: {exp.originLatencyMs}ms)
                          </p>
                        </div>
                      </div>
                      <ChevronRight className="w-4 h-4 text-slate-600 shrink-0" />
                    </div>
                  ))
                )}
              </div>
            </div>

            {/* Right Column: 5-Phase Decision Tree & Diagnostic Details */}
            <div className="col-span-7 bg-[#0b101b] border border-slate-800 rounded-xl flex flex-col overflow-hidden shadow-lg">
              {selectedExp ? (
                <div className="flex-1 overflow-y-auto p-5 space-y-6">
                  {/* Transaction Overview Header */}
                  <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                    <div>
                      <div className="flex items-center space-x-3">
                        <span
                          className={`text-xs font-mono font-bold px-2.5 py-1 rounded border uppercase ${getStatusBadge(
                            selectedExp.finalStatus
                          )}`}
                        >
                          {selectedExp.finalStatus}
                        </span>
                        <span className="font-mono text-sm text-white font-medium break-all">
                          {selectedExp.url}
                        </span>
                      </div>
                      <p className="text-xs text-slate-400 mt-1 font-mono">
                        Key: {selectedExp.primaryKey}
                      </p>
                    </div>
                    <div className="text-right shrink-0">
                      <span className="text-xs text-slate-400 block">Total Latency</span>
                      <span className="text-sm font-mono font-bold text-sky-400">
                        {selectedExp.latencyMs}ms
                      </span>
                    </div>
                  </div>

                  {/* Recommendations Alert Box */}
                  {selectedExp.recommendations && selectedExp.recommendations.length > 0 && (
                    <div className="space-y-2">
                      <h4 className="text-xs font-semibold text-slate-300 uppercase tracking-wider flex items-center space-x-1.5">
                        <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
                        <span>Actionable Optimizations &amp; Warnings</span>
                      </h4>
                      {selectedExp.recommendations.map((rec, i) => (
                        <div
                          key={i}
                          className={`p-3 rounded-lg border text-xs ${
                            rec.level === 'critical'
                              ? 'bg-rose-500/10 border-rose-500/30 text-rose-300'
                              : rec.level === 'warning'
                              ? 'bg-amber-500/10 border-amber-500/30 text-amber-300'
                              : 'bg-sky-500/10 border-sky-500/30 text-sky-300'
                          }`}
                        >
                          <div className="font-semibold">{rec.title}</div>
                          <div className="mt-1 text-slate-300">{rec.message}</div>
                        </div>
                      ))}
                    </div>
                  )}

                  {/* 5-Phase RFC 9111 Decision Explainer Pipeline */}
                  <div>
                    <h4 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-3 flex items-center space-x-1.5">
                      <Sliders className="w-3.5 h-3.5 text-sky-400" />
                      <span>RFC 9111 Step-by-Step Decision Pipeline</span>
                    </h4>
                    <div className="space-y-3">
                      {selectedExp.phases.map((phase, idx) => (
                        <div
                          key={idx}
                          className="bg-slate-900/60 border border-slate-800 rounded-lg p-3.5"
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center space-x-2">
                              {phase.passed ? (
                                <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                              ) : (
                                <XCircle className="w-4 h-4 text-rose-400 shrink-0" />
                              )}
                              <span className="text-xs font-semibold text-white">
                                {phase.name}
                              </span>
                            </div>
                            <span
                              className={`text-[10px] font-mono px-2 py-0.5 rounded ${
                                phase.passed
                                  ? 'bg-emerald-500/10 text-emerald-400'
                                  : 'bg-rose-500/10 text-rose-400'
                              }`}
                            >
                              {phase.passed ? 'PASSED' : 'CHECK'}
                            </span>
                          </div>
                          <p className="text-xs text-slate-300 mt-1.5 pl-6">
                            {phase.description}
                          </p>
                          {phase.details && phase.details.length > 0 && (
                            <ul className="mt-2 pl-6 space-y-1">
                              {phase.details.map((detail, di) => (
                                <li
                                  key={di}
                                  className="text-[11px] font-mono text-slate-400 list-disc list-inside"
                                >
                                  {detail}
                                </li>
                              ))}
                            </ul>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>

                  {/* Headers Inspector */}
                  <div>
                    <h4 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 flex items-center space-x-1.5">
                      <FileCode className="w-3.5 h-3.5 text-purple-400" />
                      <span>Edge Response Headers</span>
                    </h4>
                    <div className="bg-slate-950 border border-slate-800/80 rounded-lg p-3 font-mono text-xs text-slate-300 space-y-1 overflow-x-auto">
                      {Object.entries(selectedExp.responseHeaders).map(([k, vv]) => (
                        <div key={k} className="flex">
                          <span className="text-sky-400 font-semibold w-48 shrink-0">{k}:</span>
                          <span className="text-slate-200 break-all">{vv.join(', ')}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="p-8 text-center text-slate-500 text-xs my-auto">
                  Select a request on the left to inspect its complete RFC 9111 decision pipeline.
                </div>
              )}
            </div>
          </div>
        )}

        {/* TAB 2: SURROGATE KEYS & CACHE TAGS */}
        {activeTab === 'surrogate' && (
          <div className="flex-1 space-y-6 overflow-y-auto">
            {/* Purge Control Box */}
            <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg">
              <h3 className="text-sm font-semibold text-white mb-2 flex items-center space-x-2">
                <Trash2 className="w-4 h-4 text-rose-400" />
                <span>Trigger Invalidation (Purge Engine)</span>
              </h3>
              <p className="text-xs text-slate-400 mb-4">
                Purge distributed cache entries selectively by Tag (Surrogate-Key / Cache-Tag) or URL pattern.
              </p>

              <div className="flex flex-col md:flex-row gap-3 items-center">
                <input
                  type="text"
                  value={purgeTagInput}
                  onChange={(e) => setPurgeTagInput(e.target.value)}
                  placeholder="Enter tag (e.g. promo:blackfriday or product:42)"
                  className="flex-1 bg-slate-900 border border-slate-700 rounded-lg px-3.5 py-2 text-xs font-mono text-white placeholder-slate-500 focus:outline-none focus:border-rose-500"
                />
                <label className="flex items-center space-x-2 text-xs text-slate-300 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={isSoftPurge}
                    onChange={(e) => setIsSoftPurge(e.target.checked)}
                    className="rounded border-slate-700 text-sky-500 focus:ring-0"
                  />
                  <span>Soft Purge (Serve stale during SWR)</span>
                </label>
                <button
                  onClick={() => executePurge('tag', purgeTagInput)}
                  disabled={!purgeTagInput}
                  className="bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium px-4 py-2 rounded-lg transition disabled:opacity-50 flex items-center space-x-1.5"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  <span>Purge Tag</span>
                </button>
                <button
                  onClick={() => executePurge('all', '*')}
                  className="bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 text-xs font-medium px-4 py-2 rounded-lg transition"
                >
                  Purge All
                </button>
              </div>
            </div>

            {/* Active Tags Matrix */}
            <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg">
              <h3 className="text-sm font-semibold text-white mb-3 flex items-center space-x-2">
                <Layers className="w-4 h-4 text-sky-400" />
                <span>Active Surrogate Keys &amp; Blast Radius Index</span>
              </h3>
              {tags.length === 0 ? (
                <div className="p-8 text-center text-slate-500 text-xs">
                  No surrogate keys recorded yet. Hit <code>/mock/scenarios/product/42</code> to populate tags!
                </div>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {tags.map((t) => (
                    <div
                      key={t.tag}
                      className="bg-slate-900/60 border border-slate-800 rounded-lg p-3.5 flex flex-col justify-between"
                    >
                      <div>
                        <div className="flex items-center justify-between">
                          <span className="font-mono text-xs font-bold text-sky-400 bg-sky-950/60 border border-sky-800/40 px-2 py-0.5 rounded">
                            {t.tag}
                          </span>
                          <span className="text-[11px] font-mono text-slate-400">
                            {t.entriesCount} {t.entriesCount === 1 ? 'entry' : 'entries'}
                          </span>
                        </div>
                        <div className="mt-2 text-[11px] text-slate-400 font-mono truncate">
                          {t.sampleKeys.join(', ')}
                        </div>
                      </div>
                      <div className="mt-3 pt-2 border-t border-slate-800/60 flex items-center justify-end space-x-2">
                        <button
                          onClick={() => executePurge('tag', t.tag, true)}
                          className="text-[11px] text-sky-400 hover:text-sky-300"
                        >
                          Soft Purge
                        </button>
                        <span className="text-slate-600">&bull;</span>
                        <button
                          onClick={() => executePurge('tag', t.tag, false)}
                          className="text-[11px] text-rose-400 hover:text-rose-300"
                        >
                          Hard Purge
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Invalidation History Table */}
            <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg">
              <h3 className="text-sm font-semibold text-white mb-3">Recent Invalidation Audit Log</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="text-[11px] uppercase font-mono text-slate-400 border-b border-slate-800">
                    <tr>
                      <th className="py-2">Time</th>
                      <th className="py-2">Target</th>
                      <th className="py-2">Mode</th>
                      <th className="py-2">Affected Keys</th>
                      <th className="py-2">Duration</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60 font-mono">
                    {purgeHistory.length === 0 ? (
                      <tr>
                        <td colSpan={5} className="py-4 text-center text-slate-500">
                          No purges logged yet.
                        </td>
                      </tr>
                    ) : (
                      purgeHistory.map((h) => (
                        <tr key={h.id} className="hover:bg-slate-900/40">
                          <td className="py-2 text-slate-400">
                            {new Date(h.timestamp).toLocaleTimeString()}
                          </td>
                          <td className="py-2 text-sky-400">{h.target}</td>
                          <td className="py-2">
                            <span
                              className={`px-1.5 py-0.5 rounded text-[10px] ${
                                h.isSoft
                                  ? 'bg-cyan-500/10 text-cyan-400'
                                  : 'bg-rose-500/10 text-rose-400'
                              }`}
                            >
                              {h.isSoft ? 'SOFT' : 'HARD'}
                            </span>
                          </td>
                          <td className="py-2 text-slate-300">{h.affectedCount} entries</td>
                          <td className="py-2 text-slate-400">{h.durationMs}ms</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {/* TAB 3: CACHE STAMPEDE (DOG-PILING) LAB */}
        {activeTab === 'stampede' && (
          <div className="flex-1 space-y-6 overflow-y-auto">
            {/* Control Panel */}
            <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg">
              <div className="flex items-center space-x-2 text-amber-400 mb-1">
                <Flame className="w-5 h-5" />
                <h3 className="text-base font-semibold text-white">
                  Cache Stampede &amp; Single-Flight Coalescing Lab
                </h3>
              </div>
              <p className="text-xs text-slate-400 mb-4">
                Simulates hundreds of concurrent requests in the exact millisecond TTL expires to measure origin collapse vs single-flight request collapsing.
              </p>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
                <div>
                  <label className="text-xs text-slate-400 block mb-1">Target Endpoint</label>
                  <input
                    type="text"
                    value={stampedeUrl}
                    onChange={(e) => setStampedeUrl(e.target.value)}
                    className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-1.5 text-xs font-mono text-white"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-400 block mb-1">
                    Concurrency Level ({concurrency} simultaneous requests)
                  </label>
                  <input
                    type="range"
                    min="10"
                    max="300"
                    step="10"
                    value={concurrency}
                    onChange={(e) => setConcurrency(parseInt(e.target.value))}
                    className="w-full accent-amber-500"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-400 block mb-1">Request Collapsing</label>
                  <button
                    onClick={() => setCoalesceEnabled(!coalesceEnabled)}
                    className={`w-full py-1.5 px-3 rounded-lg text-xs font-semibold border flex items-center justify-center space-x-2 transition ${
                      coalesceEnabled
                        ? 'bg-emerald-500/10 border-emerald-500/40 text-emerald-400'
                        : 'bg-rose-500/10 border-rose-500/40 text-rose-400'
                    }`}
                  >
                    <span>{coalesceEnabled ? 'COALESCING ENABLED' : 'COALESCING DISABLED (STAMPEDE)'}</span>
                  </button>
                </div>
              </div>

              <button
                onClick={runStampedeTest}
                disabled={runningStampede}
                className="bg-amber-600 hover:bg-amber-500 text-white font-medium text-xs px-5 py-2.5 rounded-lg flex items-center space-x-2 transition disabled:opacity-50"
              >
                {runningStampede ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Play className="w-4 h-4 fill-current" />
                )}
                <span>Run Stampede Simulation</span>
              </button>
            </div>

            {/* Simulation Results Dashboard */}
            {stampedeReport && (
              <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg space-y-5">
                <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                  <div>
                    <h4 className="text-sm font-semibold text-white">Stampede Benchmark Results</h4>
                    <p className="text-xs text-slate-400 mt-0.5">{stampedeReport.recommendationNotes}</p>
                  </div>
                  <div className="text-right">
                    <span className="text-xs text-slate-400 block">Resilience Score</span>
                    <span
                      className={`text-xl font-bold font-mono ${
                        stampedeReport.resilienceScore >= 80
                          ? 'text-emerald-400'
                          : 'text-rose-400'
                      }`}
                    >
                      {stampedeReport.resilienceScore} / 100
                    </span>
                  </div>
                </div>

                {/* Scorecards */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
                  <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg">
                    <span className="text-slate-400 block">Total Requests</span>
                    <span className="text-base font-bold font-mono text-white">
                      {stampedeReport.concurrency}
                    </span>
                  </div>
                  <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg">
                    <span className="text-slate-400 block">Origin Hits (Penetration)</span>
                    <span
                      className={`text-base font-bold font-mono ${
                        stampedeReport.originHits > 2 ? 'text-rose-400' : 'text-emerald-400'
                      }`}
                    >
                      {stampedeReport.originHits} ({stampedeReport.penetrationRate.toFixed(1)}%)
                    </span>
                  </div>
                  <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg">
                    <span className="text-slate-400 block">Edge Hits (Absorbed)</span>
                    <span className="text-base font-bold font-mono text-sky-400">
                      {stampedeReport.edgeHits}
                    </span>
                  </div>
                  <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg">
                    <span className="text-slate-400 block">P95 Latency</span>
                    <span className="text-base font-bold font-mono text-amber-400">
                      {stampedeReport.percentiles.p95Ms}ms
                    </span>
                  </div>
                </div>

                {/* Percentiles Bar */}
                <div className="bg-slate-950 p-3.5 rounded-lg border border-slate-800 text-xs font-mono grid grid-cols-5 gap-2 text-center">
                  <div>
                    <span className="text-slate-500 block text-[10px]">MIN</span>
                    <span className="text-slate-200">{stampedeReport.percentiles.minMs}ms</span>
                  </div>
                  <div>
                    <span className="text-slate-500 block text-[10px]">P50 (MEDIAN)</span>
                    <span className="text-slate-200">{stampedeReport.percentiles.p50Ms}ms</span>
                  </div>
                  <div>
                    <span className="text-slate-500 block text-[10px]">P90</span>
                    <span className="text-slate-200">{stampedeReport.percentiles.p90Ms}ms</span>
                  </div>
                  <div>
                    <span className="text-slate-500 block text-[10px]">P99</span>
                    <span className="text-slate-200">{stampedeReport.percentiles.p99Ms}ms</span>
                  </div>
                  <div>
                    <span className="text-slate-500 block text-[10px]">MAX</span>
                    <span className="text-slate-200">{stampedeReport.percentiles.maxMs}ms</span>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* TAB 4: CDN FLAVOR EMULATION */}
        {activeTab === 'cdn' && (
          <div className="flex-1 space-y-6 overflow-y-auto">
            <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg">
              <h3 className="text-sm font-semibold text-white mb-2 flex items-center space-x-2">
                <Globe className="w-4 h-4 text-sky-400" />
                <span>Multi-CDN Edge Header Emulation</span>
              </h3>
              <p className="text-xs text-slate-400 mb-5">
                Switch the active edge engine profile to test header behavior across major CDN platforms without deploying.
              </p>

              <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-5 gap-3">
                {[
                  {
                    id: 'cloudflare',
                    name: 'Cloudflare',
                    header: 'cf-cache-status: HIT | MISS | STALE | DYNAMIC',
                    desc: 'Injects cf-ray, cf-cache-status, and Cloudflare default caching extensions.',
                  },
                  {
                    id: 'fastly',
                    name: 'Fastly',
                    header: 'X-Cache: HIT | MISS, X-Cache-Hits',
                    desc: 'Prioritizes Surrogate-Control, manages Surrogate-Key, strips internal tags on egress.',
                  },
                  {
                    id: 'cloudfront',
                    name: 'AWS CloudFront',
                    header: 'X-Cache: Hit from cloudfront',
                    desc: 'Emulates CloudFront cache behaviors, Via headers, and POP identifiers.',
                  },
                  {
                    id: 'vercel',
                    name: 'Vercel Edge',
                    header: 'x-vercel-cache: HIT | MISS | STALE',
                    desc: 'Vercel edge caching conventions with SWR background revalidation triggers.',
                  },
                  {
                    id: 'rfc9111',
                    name: 'RFC 9111 Generic',
                    header: 'Cache-Status, Age, Warning',
                    desc: 'Strict RFC 9111 and RFC 9211 standards-compliant caching headers.',
                  },
                ].map((f) => (
                  <button
                    key={f.id}
                    onClick={() => switchCDNFlavor(f.id as CDNFlavor)}
                    className={`p-4 rounded-xl border text-left transition ${
                      cdnConfig?.activeFlavor === f.id
                        ? 'bg-sky-500/10 border-sky-500 text-white shadow-lg shadow-sky-500/10'
                        : 'bg-slate-900/60 border-slate-800 text-slate-400 hover:border-slate-700'
                    }`}
                  >
                    <div className="flex items-center justify-between mb-2">
                      <span className="font-semibold text-sm text-white">{f.name}</span>
                      {cdnConfig?.activeFlavor === f.id && (
                        <CheckCircle2 className="w-4 h-4 text-sky-400" />
                      )}
                    </div>
                    <p className="text-[11px] font-mono text-sky-400/90 mb-2">{f.header}</p>
                    <p className="text-xs text-slate-400 leading-relaxed">{f.desc}</p>
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* TAB 5: MOCK ORIGIN SANDBOX & SCENARIOS */}
        {activeTab === 'mock' && (
          <div className="flex-1 space-y-6 overflow-y-auto">
            <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg">
              <h3 className="text-sm font-semibold text-white mb-2 flex items-center space-x-2">
                <Server className="w-4 h-4 text-sky-400" />
                <span>Embedded Mock Origin Sandbox &amp; Scenarios</span>
              </h3>
              <p className="text-xs text-slate-400 mb-5">
                Pre-configured edge caching challenge scenarios. Click any scenario to run a live test through the proxy.
              </p>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {mockRoutes.map((r) => (
                  <div
                    key={r.id}
                    className="bg-slate-900/70 border border-slate-800 rounded-xl p-4 flex flex-col justify-between"
                  >
                    <div>
                      <div className="flex items-center justify-between mb-1.5">
                        <span className="text-xs font-mono font-bold text-sky-400">{r.path}</span>
                        <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-800 text-slate-300">
                          {r.hitCount} origin hits
                        </span>
                      </div>
                      <p className="text-xs text-slate-300 mb-3">{r.description}</p>
                      <div className="bg-slate-950 p-2 rounded border border-slate-800/80 font-mono text-[11px] text-slate-400 space-y-0.5 mb-3">
                        {Object.entries(r.headers).map(([k, v]) => (
                          <div key={k} className="truncate">
                            <span className="text-sky-400">{k}:</span> {v}
                          </div>
                        ))}
                      </div>
                    </div>
                    <button
                      onClick={() => {
                        setTestUrl(r.path);
                        triggerTestRequest(r.path);
                        setActiveTab('traffic');
                      }}
                      className="bg-sky-500/10 hover:bg-sky-500/20 border border-sky-500/30 text-sky-400 font-medium text-xs py-1.5 px-3 rounded-lg flex items-center justify-center space-x-1.5 transition"
                    >
                      <Play className="w-3 h-3 fill-current" />
                      <span>Test Scenario via Edge Proxy</span>
                    </button>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* TAB 6: EDGE HEADER LINTER & AUDITOR */}
        {activeTab === 'audit' && (
          <div className="flex-1 space-y-6 overflow-y-auto">
            <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg">
              <h3 className="text-sm font-semibold text-white mb-2 flex items-center space-x-2">
                <Shield className="w-4 h-4 text-sky-400" />
                <span>RFC 9111 Edge Header Compliance &amp; Security Auditor</span>
              </h3>
              <p className="text-xs text-slate-400 mb-4">
                Paste HTTP response headers or enter a public URL to grade caching efficiency, detect cookie leaks, and verify compliance.
              </p>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                <div>
                  <label className="text-xs text-slate-400 block mb-1">Target Live URL (optional)</label>
                  <input
                    type="text"
                    value={auditUrl}
                    onChange={(e) => setAuditUrl(e.target.value)}
                    placeholder="https://example.com/api/data"
                    className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-1.5 text-xs font-mono text-white placeholder-slate-500"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-400 block mb-1">Or Paste Response Headers</label>
                  <textarea
                    rows={4}
                    value={customHeaders}
                    onChange={(e) => setCustomHeaders(e.target.value)}
                    className="w-full bg-slate-900 border border-slate-700 rounded-lg p-2.5 text-xs font-mono text-white"
                  />
                </div>
              </div>

              <button
                onClick={runAudit}
                disabled={auditing}
                className="bg-sky-500 hover:bg-sky-400 text-white font-medium text-xs px-5 py-2 rounded-lg transition disabled:opacity-50"
              >
                {auditing ? 'Auditing Headers...' : 'Run Header Audit'}
              </button>
            </div>

            {auditReport && (
              <div className="bg-[#0b101b] border border-slate-800 rounded-xl p-5 shadow-lg space-y-4">
                <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                  <div>
                    <h4 className="text-sm font-semibold text-white">Header Compliance Grade</h4>
                    <p className="text-xs text-slate-400">Score: {auditReport.score} / 100</p>
                  </div>
                  <span
                    className={`text-2xl font-bold font-mono px-3 py-1 rounded border ${
                      auditReport.grade === 'A+' || auditReport.grade === 'A'
                        ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                        : auditReport.grade === 'B' || auditReport.grade === 'C'
                        ? 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                        : 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                    }`}
                  >
                    {auditReport.grade}
                  </span>
                </div>

                <div className="space-y-3">
                  {auditReport.findings.map((f, i) => (
                    <div
                      key={i}
                      className={`p-3.5 rounded-lg border text-xs ${
                        f.severity === 'critical'
                          ? 'bg-rose-500/10 border-rose-500/30 text-rose-300'
                          : f.severity === 'high' || f.severity === 'medium'
                          ? 'bg-amber-500/10 border-amber-500/30 text-amber-300'
                          : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-semibold">{f.title}</span>
                        <span className="text-[10px] font-mono uppercase">{f.ruleId}</span>
                      </div>
                      <p className="text-slate-300 mt-1">{f.description}</p>
                      <p className="text-sky-300 font-mono text-[11px] mt-1.5">
                        Suggestion: {f.suggestion}
                      </p>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
export default App;
