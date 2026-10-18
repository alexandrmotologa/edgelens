import React, { useState, useEffect } from 'react';
import { Columns, Play, AlertCircle, CheckCircle2, ArrowRight, ShieldAlert, Sparkles, Copy, Check } from 'lucide-react';
import { ComparisonReport } from '../types';

export function ComparatorTab() {
  const [url, setUrl] = useState('/mock/scenarios/product/42');
  const [method, setMethod] = useState('GET');
  const [customHeaders, setCustomHeaders] = useState('Cache-Control: public, s-maxage=3600, max-age=600\nSurrogate-Control: max-age=86400\nVary: Accept-Encoding\nETag: "prod-42-v1"');
  const [statusCode, setStatusCode] = useState(200);
  const [simulatedAge, setSimulatedAge] = useState(120);
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState<ComparisonReport | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    runComparison();
  }, []);

  const runComparison = async () => {
    setLoading(true);
    try {
      const respHeaders: Record<string, string[]> = {};
      customHeaders.split('\n').forEach(line => {
        const idx = line.indexOf(':');
        if (idx !== -1) {
          const k = line.slice(0, idx).trim();
          const v = line.slice(idx + 1).trim();
          if (!respHeaders[k]) respHeaders[k] = [];
          respHeaders[k].push(v);
        }
      });

      const res = await fetch('/api/compare', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url,
          method,
          statusCode: Number(statusCode),
          responseHeaders: respHeaders,
          simulatedAgeSecs: Number(simulatedAge),
        }),
      });

      if (res.ok) {
        setReport(await res.json());
      }
    } catch (err) {
      console.error('Comparison error:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleCopyJSON = () => {
    if (!report) return;
    navigator.clipboard.writeText(JSON.stringify(report, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="space-y-6">
      {/* Configuration Header */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-indigo-500/10 border border-indigo-500/20 text-indigo-400">
              <Columns size={20} />
            </div>
            <div>
              <h2 className="text-base font-semibold text-white">Multi-CDN Side-by-Side Comparator</h2>
              <p className="text-xs text-slate-400">
                Evaluate cache eligibility, TTL calculations, and header behavior across 5 edge networks simultaneously.
              </p>
            </div>
          </div>
          {report && (
            <button
              onClick={handleCopyJSON}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 transition whitespace-nowrap shrink-0"
            >
              {copied ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
              {copied ? 'Copied Report' : 'Copy JSON'}
            </button>
          )}
        </div>

        <div className="grid grid-cols-1 md:grid-cols-12 gap-3 mb-4">
          <div className="md:col-span-2">
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Method</label>
            <select
              value={method}
              onChange={e => setMethod(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
            >
              <option value="GET">GET</option>
              <option value="HEAD">HEAD</option>
              <option value="POST">POST</option>
            </select>
          </div>
          <div className="md:col-span-6">
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Target URL</label>
            <input
              type="text"
              value={url}
              onChange={e => setUrl(e.target.value)}
              placeholder="https://api.example.com/v1/products"
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-white placeholder-slate-600 focus:outline-none focus:border-indigo-500"
            />
          </div>
          <div className="md:col-span-2">
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Status Code</label>
            <input
              type="number"
              value={statusCode}
              onChange={e => setStatusCode(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
            />
          </div>
          <div className="md:col-span-2">
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Simulated Age</label>
            <input
              type="number"
              value={simulatedAge}
              onChange={e => setSimulatedAge(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
            />
          </div>
        </div>

        <div className="mb-4">
          <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">
            Simulated Origin Response Headers (One per line)
          </label>
          <textarea
            rows={3}
            value={customHeaders}
            onChange={e => setCustomHeaders(e.target.value)}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-emerald-400 focus:outline-none focus:border-indigo-500"
          />
        </div>

        <button
          onClick={runComparison}
          disabled={loading}
          className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium transition shadow-lg shadow-indigo-600/20 disabled:opacity-50 whitespace-nowrap"
        >
          <Play size={14} />
          {loading ? 'Evaluating Flavors...' : 'Compare Across All CDNs'}
        </button>
      </div>

      {report && (
        <div className="space-y-6">
          {/* CDN Grid */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
            {Object.entries(report.flavors).map(([key, f]) => {
              const outcomeColors: Record<string, string> = {
                HIT: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
                MISS: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
                STALE: 'bg-sky-500/10 text-sky-400 border-sky-500/20',
                BYPASS: 'bg-purple-500/10 text-purple-400 border-purple-500/20',
                DYNAMIC: 'bg-rose-500/10 text-rose-400 border-rose-500/20',
              };

              return (
                <div key={key} className="bg-slate-900 border border-slate-800 rounded-xl p-4 flex flex-col justify-between">
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-xs font-bold text-white uppercase tracking-wider">{f.flavorName}</span>
                      <span className={`text-[10px] font-bold px-2 py-0.5 rounded border ${outcomeColors[f.cacheOutcome] || 'bg-slate-800 text-slate-300'}`}>
                        {f.cacheOutcome}
                      </span>
                    </div>

                    <div className="space-y-2 mt-3 text-xs">
                      <div className="bg-slate-950 rounded-lg p-2 border border-slate-800/80">
                        <span className="text-[10px] text-slate-500 block">Status Header</span>
                        <code className="text-indigo-400 font-mono text-[11px] block truncate">
                          {f.statusHeader}: {f.statusValue}
                        </code>
                      </div>

                      <div className="bg-slate-950 rounded-lg p-2 border border-slate-800/80">
                        <span className="text-[10px] text-slate-500 block">Calculated TTL</span>
                        <span className="text-white font-mono font-semibold">{f.effectiveTtlSeconds} seconds</span>
                      </div>

                      <div className="bg-slate-950 rounded-lg p-2 border border-slate-800/80">
                        <span className="text-[10px] text-slate-500 block">Surrogate Precedence</span>
                        <span className="text-slate-300 text-[11px] leading-tight block">{f.surrogatePrecedence}</span>
                      </div>
                    </div>
                  </div>

                  {f.keyNotes.length > 0 && (
                    <div className="mt-3 pt-3 border-t border-slate-800 text-[11px] text-slate-400 space-y-1">
                      {f.keyNotes.map((note, idx) => (
                        <p key={idx} className="flex items-start gap-1">
                          <span className="text-indigo-400">•</span> {note}
                        </p>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {/* Divergences & Differences */}
          {report.differences.length > 0 && (
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
              <div className="flex items-center gap-2 mb-4">
                <ShieldAlert size={18} className="text-amber-400" />
                <h3 className="text-sm font-semibold text-white">Detected CDN Divergences ({report.differences.length})</h3>
              </div>

              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-slate-950 text-slate-400 border-b border-slate-800 uppercase text-[10px] tracking-wider">
                    <tr>
                      <th className="py-2.5 px-3">Field</th>
                      <th className="py-2.5 px-3">Description</th>
                      <th className="py-2.5 px-3">Values Across CDNs</th>
                      <th className="py-2.5 px-3">Impact</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60 font-mono text-[11px]">
                    {report.differences.map((diff, idx) => (
                      <tr key={idx} className="hover:bg-slate-800/30">
                        <td className="py-3 px-3 text-white font-sans font-semibold">{diff.field}</td>
                        <td className="py-3 px-3 text-slate-400 font-sans">{diff.description}</td>
                        <td className="py-3 px-3">
                          <div className="space-y-1">
                            {Object.entries(diff.values || {}).map(([k, v]) => (
                              <div key={k} className="flex items-center gap-2">
                                <span className="text-slate-500 uppercase text-[10px]">{k}:</span>
                                <span className="text-emerald-400">{String(v)}</span>
                              </div>
                            ))}
                          </div>
                        </td>
                        <td className="py-3 px-3 text-amber-400 font-sans font-medium">{diff.impact}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Recommendations */}
          {report.recommendations.length > 0 && (
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
              <div className="flex items-center gap-2 mb-3">
                <Sparkles size={18} className="text-indigo-400" />
                <h3 className="text-sm font-semibold text-white">Edge Alignment Recommendations</h3>
              </div>
              <ul className="space-y-2 text-xs text-slate-300">
                {report.recommendations.map((rec, idx) => (
                  <li key={idx} className="flex items-start gap-2">
                    <CheckCircle2 size={14} className="text-emerald-400 mt-0.5 shrink-0" />
                    <span>{rec}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
