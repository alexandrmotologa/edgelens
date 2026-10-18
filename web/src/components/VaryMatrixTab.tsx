import React, { useState } from 'react';
import { Layers, AlertTriangle, CheckCircle2, ArrowRight, Zap, Copy, Check, Flame } from 'lucide-react';
import { VaryAnalysisReport } from '../types';

export function VaryMatrixTab() {
  const [varyHeader, setVaryHeader] = useState('Accept-Encoding, User-Agent, Accept-Language');
  const [report, setReport] = useState<VaryAnalysisReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [copiedProposal, setCopiedProposal] = useState(false);

  const analyzeVary = async (headerToTest?: string) => {
    const val = headerToTest !== undefined ? headerToTest : varyHeader;
    setLoading(true);
    try {
      const res = await fetch('/api/vary-matrix', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ varyHeader: val }),
      });
      if (res.ok) {
        setReport(await res.json());
      }
    } catch (err) {
      console.error('Vary analysis error:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleCopyProposal = () => {
    if (!report) return;
    navigator.clipboard.writeText(`Vary: ${report.normalizedVaryProposal}`);
    setCopiedProposal(true);
    setTimeout(() => setCopiedProposal(false), 2000);
  };

  return (
    <div className="space-y-6">
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 rounded-lg bg-purple-500/10 border border-purple-500/20 text-purple-400">
            <Layers size={20} />
          </div>
          <div>
            <h2 className="text-base font-semibold text-white">Vary Matrix & Fragmentation Visualizer</h2>
            <p className="text-xs text-slate-400">
              Audit the combinatorial explosion caused by the HTTP Vary header and discover edge normalization rules.
            </p>
          </div>
        </div>

        <div className="space-y-3 mb-4">
          <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
            Vary Header Value
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={varyHeader}
              onChange={e => setVaryHeader(e.target.value)}
              placeholder="Accept-Encoding, User-Agent, Accept-Language"
              className="flex-1 bg-slate-950 border border-slate-800 rounded-lg px-4 py-2 text-xs font-mono text-purple-400 focus:outline-none focus:border-purple-500"
            />
            <button
              onClick={() => analyzeVary()}
              disabled={loading}
              className="flex items-center gap-2 px-5 py-2 rounded-lg bg-purple-600 hover:bg-purple-500 text-white text-xs font-medium transition shadow-lg shadow-purple-600/20 disabled:opacity-50"
            >
              <Zap size={14} />
              {loading ? 'Analyzing...' : 'Analyze Fragmentation'}
            </button>
          </div>

          {/* Quick Presets */}
          <div className="flex items-center gap-2 text-xs text-slate-400 pt-1">
            <span className="text-[11px] text-slate-500">Presets:</span>
            <button
              onClick={() => {
                setVaryHeader('Accept-Encoding, User-Agent, Cookie');
                analyzeVary('Accept-Encoding, User-Agent, Cookie');
              }}
              className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-mono transition"
            >
              Worst Case (Explosion)
            </button>
            <button
              onClick={() => {
                setVaryHeader('Accept-Encoding');
                analyzeVary('Accept-Encoding');
              }}
              className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-mono transition"
            >
              Modern RFC Best Practice
            </button>
            <button
              onClick={() => {
                setVaryHeader('Accept-Encoding, Accept-Language');
                analyzeVary('Accept-Encoding, Accept-Language');
              }}
              className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-mono transition"
            >
              Multi-Language API
            </button>
          </div>
        </div>
      </div>

      {report && (
        <div className="space-y-6">
          {/* Key Metrics Banner */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
              <span className="text-[11px] text-slate-400 uppercase tracking-wider block mb-1">Estimated Permutations</span>
              <span className="text-2xl font-bold font-mono text-white">{report.estimatedPermutations.toLocaleString()}</span>
              <span className="text-[11px] text-slate-500 block mt-1">Unique cache variants per URI</span>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
              <span className="text-[11px] text-slate-400 uppercase tracking-wider block mb-1">Origin Penetration Risk</span>
              <div className="flex items-center gap-2 mt-1">
                <span
                  className={`px-2.5 py-1 rounded text-xs font-bold uppercase tracking-wider ${
                    report.originPenetrationRisk === 'CRITICAL'
                      ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30'
                      : report.originPenetrationRisk === 'HIGH'
                      ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                      : report.originPenetrationRisk === 'MEDIUM'
                      ? 'bg-blue-500/20 text-blue-400 border border-blue-500/30'
                      : 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                  }`}
                >
                  {report.originPenetrationRisk} RISK
                </span>
              </div>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
              <span className="text-[11px] text-slate-400 uppercase tracking-wider block mb-1">Memory Multiplier</span>
              <span className="text-2xl font-bold font-mono text-purple-400">{report.estimatedMemoryMultiplier.toLocaleString()}x</span>
              <span className="text-[11px] text-slate-500 block mt-1">Base storage overhead</span>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 flex flex-col justify-between">
              <div>
                <span className="text-[11px] text-slate-400 uppercase tracking-wider block mb-1">Recommended Proposal</span>
                <code className="text-xs font-mono text-emerald-400 block truncate">{report.normalizedVaryProposal}</code>
              </div>
              <button
                onClick={handleCopyProposal}
                className="flex items-center gap-1.5 text-[11px] font-medium text-slate-300 hover:text-white transition mt-2"
              >
                {copiedProposal ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
                {copiedProposal ? 'Copied to Clipboard' : 'Copy Header'}
              </button>
            </div>
          </div>

          {/* Dimension Breakdown */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
            <h3 className="text-sm font-semibold text-white mb-4">Vary Dimensions Breakdown</h3>
            <div className="space-y-3">
              {report.dimensions.map((dim, idx) => (
                <div key={idx} className="bg-slate-950 border border-slate-800 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-bold font-mono text-purple-400">{dim.headerName}</span>
                      <span className="text-[10px] text-slate-400 bg-slate-900 px-2 py-0.5 rounded border border-slate-800">
                        ~{dim.typicalCardinality} Cardinality
                      </span>
                    </div>
                    <span
                      className={`text-[10px] font-bold px-2 py-0.5 rounded border ${
                        dim.risk === 'CRITICAL'
                          ? 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                          : dim.risk === 'HIGH'
                          ? 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                          : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                      }`}
                    >
                      {dim.risk}
                    </span>
                  </div>

                  <p className="text-xs text-slate-300 mb-2">{dim.description}</p>
                  <div className="bg-slate-900/60 rounded p-2.5 border border-slate-800 text-xs">
                    <span className="text-[11px] font-semibold text-emerald-400 block mb-0.5">Edge Normalization Action:</span>
                    <span className="text-slate-300">{dim.edgeRemediation}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Concrete Sample Variants */}
          {report.sampleVariants.length > 0 && (
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
              <h3 className="text-sm font-semibold text-white mb-2">Simulated Cache Slices</h3>
              <p className="text-xs text-slate-400 mb-4">
                Below are real HTTP request variations hitting the edge. Notice how edge normalization collapses them back into a single unified cache entry:
              </p>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                {report.sampleVariants.map((variant, idx) => (
                  <div key={idx} className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs flex flex-col justify-between">
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <span className="font-mono text-slate-400 text-[11px]">Variant #{idx + 1}</span>
                        {variant.collapsesWithFix ? (
                          <span className="text-[10px] bg-emerald-500/10 text-emerald-400 px-1.5 py-0.5 rounded border border-emerald-500/20">
                            Collapses
                          </span>
                        ) : (
                          <span className="text-[10px] bg-slate-800 text-slate-400 px-1.5 py-0.5 rounded">Primary Key</span>
                        )}
                      </div>

                      <div className="space-y-1 font-mono text-[11px] text-slate-300">
                        {Object.entries(variant.headers).map(([k, v]) => (
                          <div key={k} className="truncate">
                            <span className="text-slate-500">{k}:</span> <span className="text-indigo-400">{v}</span>
                          </div>
                        ))}
                      </div>
                    </div>

                    <div className="mt-3 pt-2 border-t border-slate-800/80 font-mono text-[10px] text-slate-500 truncate">
                      Cache Key Suffix: <span className="text-purple-400">{variant.cacheKeySuffix}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Recommendations List */}
          {report.recommendations.length > 0 && (
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
              <h3 className="text-sm font-semibold text-white mb-3">Remediation Steps</h3>
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
