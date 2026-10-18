import React, { useState } from 'react';
import { ShieldCheck, Play, CheckCircle2, XCircle, Copy, Check, Terminal } from 'lucide-react';
import { AssertionResult } from '../types';

export function AssertTab() {
  const [url, setUrl] = useState('http://localhost:8080/mock/scenarios/product/42');
  const [expectStatus, setExpectStatus] = useState('HIT');
  const [minTtl, setMinTtl] = useState(300);
  const [minGrade, setMinGrade] = useState('B');
  const [disallowedHeaders, setDisallowedHeaders] = useState('Set-Cookie');
  const [requiredHeaders, setRequiredHeaders] = useState('Cache-Control');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<AssertionResult | null>(null);
  const [copiedCli, setCopiedCli] = useState(false);

  const runAssertion = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/assert', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url,
          expectStatus,
          minTtlSeconds: Number(minTtl),
          minGrade,
          disallowedHeaders: disallowedHeaders.split(',').map(s => s.trim()).filter(Boolean),
          requiredHeaders: requiredHeaders.split(',').map(s => s.trim()).filter(Boolean),
        }),
      });

      if (res.ok) {
        setResult(await res.json());
      }
    } catch (err) {
      console.error('Assertion error:', err);
    } finally {
      setLoading(false);
    }
  };

  const generateCliCommand = () => {
    let cmd = `edgelens assert --url "${url}"`;
    if (expectStatus) cmd += ` --expect-status "${expectStatus}"`;
    if (minTtl > 0) cmd += ` --min-ttl ${minTtl}`;
    if (minGrade) cmd += ` --min-grade "${minGrade}"`;
    if (disallowedHeaders) cmd += ` --disallow "${disallowedHeaders}"`;
    if (requiredHeaders) cmd += ` --require "${requiredHeaders}"`;
    return cmd;
  };

  const handleCopyCli = () => {
    navigator.clipboard.writeText(generateCliCommand());
    setCopiedCli(true);
    setTimeout(() => setCopiedCli(false), 2000);
  };

  return (
    <div className="space-y-6">
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 rounded-lg bg-blue-500/10 border border-blue-500/20 text-blue-400">
            <ShieldCheck size={20} />
          </div>
          <div>
            <h2 className="text-base font-semibold text-white">CI/CD Cache Assertion Runner</h2>
            <p className="text-xs text-slate-400">
              Enforce strict caching contracts in staging and production deployment pipelines. Catch accidental Set-Cookie leaks and TTL regressions before release.
            </p>
          </div>
        </div>

        {/* Assertion Form */}
        <div className="grid grid-cols-1 md:grid-cols-12 gap-3 text-xs mb-4">
          <div className="md:col-span-6">
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Target URL</label>
            <input
              type="text"
              value={url}
              onChange={e => setUrl(e.target.value)}
              placeholder="http://localhost:8080/api/v1/products"
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>
          <div className="md:col-span-2">
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Expected Status</label>
            <select
              value={expectStatus}
              onChange={e => setExpectStatus(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
            >
              <option value="">(Any)</option>
              <option value="HIT">HIT</option>
              <option value="MISS">MISS</option>
              <option value="STALE">STALE</option>
              <option value="REVALIDATED">REVALIDATED</option>
            </select>
          </div>
          <div className="md:col-span-2">
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Min Edge TTL (s)</label>
            <input
              type="number"
              value={minTtl}
              onChange={e => setMinTtl(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>
          <div className="md:col-span-2">
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Min Grade</label>
            <select
              value={minGrade}
              onChange={e => setMinGrade(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
            >
              <option value="">(Any)</option>
              <option value="A+">A+</option>
              <option value="A">A</option>
              <option value="B">B</option>
              <option value="C">C</option>
              <option value="D">D</option>
            </select>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs mb-4">
          <div>
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Disallowed Headers (Comma-Separated)</label>
            <input
              type="text"
              value={disallowedHeaders}
              onChange={e => setDisallowedHeaders(e.target.value)}
              placeholder="Set-Cookie, Authorization"
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Required Headers (Comma-Separated)</label>
            <input
              type="text"
              value={requiredHeaders}
              onChange={e => setRequiredHeaders(e.target.value)}
              placeholder="Cache-Control, ETag"
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <button
          onClick={runAssertion}
          disabled={loading}
          className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-medium transition shadow-lg shadow-blue-600/20 disabled:opacity-50"
        >
          <Play size={14} />
          {loading ? 'Evaluating Assertions...' : 'Run Assertion Check'}
        </button>
      </div>

      {/* CLI Snippet Card */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 flex items-center justify-between text-xs">
        <div className="flex items-center gap-3 overflow-hidden">
          <Terminal size={18} className="text-slate-400 shrink-0" />
          <div className="overflow-hidden">
            <span className="text-[11px] text-slate-500 block uppercase font-semibold tracking-wider">CI/CD Pipeline Command</span>
            <code className="text-blue-400 font-mono text-[11px] block truncate">{generateCliCommand()}</code>
          </div>
        </div>
        <button
          onClick={handleCopyCli}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 transition text-[11px] font-medium shrink-0"
        >
          {copiedCli ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
          {copiedCli ? 'Copied' : 'Copy CLI'}
        </button>
      </div>

      {/* Results View */}
      {result && (
        <div className="space-y-4">
          <div
            className={`rounded-xl p-5 border flex items-center justify-between ${
              result.passed
                ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-400'
                : 'bg-rose-500/10 border-rose-500/20 text-rose-400'
            }`}
          >
            <div className="flex items-center gap-3">
              {result.passed ? <CheckCircle2 size={24} /> : <XCircle size={24} />}
              <div>
                <h3 className="text-sm font-bold uppercase tracking-wider">
                  {result.passed ? 'Assertion Suite PASSED' : 'Assertion Suite FAILED'}
                </h3>
                <p className="text-xs text-slate-300 mt-0.5">{result.summary}</p>
              </div>
            </div>

            <div className="flex items-center gap-4 text-xs font-mono">
              <div>
                <span className="text-slate-400 text-[10px] block uppercase">Grade</span>
                <span className="text-white font-bold">{result.auditGrade} ({result.auditScore}/100)</span>
              </div>
              <div>
                <span className="text-slate-400 text-[10px] block uppercase">TTL</span>
                <span className="text-white font-bold">{result.calculatedTtl}s</span>
              </div>
              <div>
                <span className="text-slate-400 text-[10px] block uppercase">Latency</span>
                <span className="text-white font-bold">{result.durationMs}ms</span>
              </div>
            </div>
          </div>

          {/* Checks Table */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-lg">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950 text-slate-400 border-b border-slate-800 uppercase text-[10px] tracking-wider">
                <tr>
                  <th className="py-2.5 px-4">Status</th>
                  <th className="py-2.5 px-4">Assertion Check</th>
                  <th className="py-2.5 px-4">Expected</th>
                  <th className="py-2.5 px-4">Actual Value</th>
                  <th className="py-2.5 px-4">Details</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 font-mono text-[11px]">
                {result.checks.map((chk, idx) => (
                  <tr key={idx} className="hover:bg-slate-800/30">
                    <td className="py-3 px-4">
                      {chk.passed ? (
                        <span className="flex items-center gap-1.5 text-emerald-400 font-sans font-semibold">
                          <CheckCircle2 size={16} /> PASS
                        </span>
                      ) : (
                        <span className="flex items-center gap-1.5 text-rose-400 font-sans font-semibold">
                          <XCircle size={16} /> FAIL
                        </span>
                      )}
                    </td>
                    <td className="py-3 px-4 text-white font-sans font-medium">{chk.name}</td>
                    <td className="py-3 px-4 text-slate-400">{chk.expected}</td>
                    <td className="py-3 px-4 text-indigo-400">{chk.actual}</td>
                    <td className="py-3 px-4 font-sans text-xs">
                      {chk.errorMessage ? (
                        <span className="text-rose-400">{chk.errorMessage}</span>
                      ) : (
                        <span className="text-slate-500">Criteria satisfied</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
