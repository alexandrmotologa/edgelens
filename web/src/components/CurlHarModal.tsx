import React, { useState } from 'react';
import { X, Terminal, FileText, Upload, CheckCircle2, AlertTriangle, ArrowRight, Play } from 'lucide-react';
import { CurlParsedRequest, HARReport } from '../types';

interface CurlHarModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSelectUrl: (url: string) => void;
}

export function CurlHarModal({ isOpen, onClose, onSelectUrl }: CurlHarModalProps) {
  const [activeTab, setActiveTab] = useState<'curl' | 'har'>('curl');

  // cURL state
  const [curlCommand, setCurlCommand] = useState('curl -X GET "https://api.example.com/v1/products" \\\n  -H "Accept: application/json" \\\n  -H "Cache-Control: no-cache"');
  const [parsedCurl, setParsedCurl] = useState<CurlParsedRequest | null>(null);
  const [curlError, setCurlError] = useState<string | null>(null);
  const [loadingCurl, setLoadingCurl] = useState(false);

  // HAR state
  const [harText, setHarText] = useState('');
  const [harReport, setHarReport] = useState<HARReport | null>(null);
  const [harError, setHarError] = useState<string | null>(null);
  const [loadingHar, setLoadingHar] = useState(false);

  if (!isOpen) return null;

  const handleParseCurl = async () => {
    setLoadingCurl(true);
    setCurlError(null);
    try {
      const res = await fetch('/api/parse-curl', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command: curlCommand }),
      });
      if (res.ok) {
        setParsedCurl(await res.json());
      } else {
        const text = await res.text();
        setCurlError(text || 'Failed to parse cURL command');
      }
    } catch (err: any) {
      setCurlError(err.message);
    } finally {
      setLoadingCurl(false);
    }
  };

  const handleParseHAR = async (content?: string) => {
    const payload = content !== undefined ? content : harText;
    if (!payload.trim()) return;

    setLoadingHar(true);
    setHarError(null);
    try {
      const res = await fetch('/api/parse-har', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
      });
      if (res.ok) {
        setHarReport(await res.json());
      } else {
        const text = await res.text();
        setHarError(text || 'Failed to parse HAR JSON');
      }
    } catch (err: any) {
      setHarError(err.message);
    } finally {
      setLoadingHar(false);
    }
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      const text = event.target?.result as string;
      setHarText(text);
      handleParseHAR(text);
    };
    reader.readAsText(file);
  };

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-4xl w-full max-h-[90vh] flex flex-col shadow-2xl overflow-hidden">
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950">
          <div className="flex items-center gap-2 overflow-x-auto scrollbar-none">
            <button
              onClick={() => setActiveTab('curl')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition shrink-0 whitespace-nowrap ${
                activeTab === 'curl' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-white'
              }`}
            >
              <Terminal size={14} />
              Import cURL Command
            </button>
            <button
              onClick={() => setActiveTab('har')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition shrink-0 whitespace-nowrap ${
                activeTab === 'har' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-white'
              }`}
            >
              <FileText size={14} />
              Import DevTools HAR Waterfall
            </button>
          </div>

          <button onClick={onClose} className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition">
            <X size={18} />
          </button>
        </div>

        {/* Modal Content */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
          {activeTab === 'curl' ? (
            <div className="space-y-4">
              <div>
                <label className="block text-slate-400 mb-1 font-semibold uppercase text-[10px] tracking-wider">
                  Paste cURL Command
                </label>
                <textarea
                  rows={4}
                  value={curlCommand}
                  onChange={e => setCurlCommand(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-emerald-400 font-mono text-xs focus:outline-none focus:border-indigo-500"
                />
              </div>

              {curlError && (
                <div className="bg-rose-500/10 border border-rose-500/20 text-rose-400 p-3 rounded-lg flex items-center gap-2">
                  <AlertTriangle size={16} />
                  <span>{curlError}</span>
                </div>
              )}

              <button
                onClick={handleParseCurl}
                disabled={loadingCurl}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium transition shadow-lg shadow-indigo-600/20 whitespace-nowrap"
              >
                <Play size={14} />
                {loadingCurl ? 'Parsing...' : 'Parse & Extract Headers'}
              </button>

              {parsedCurl && (
                <div className="bg-slate-950 border border-slate-800 rounded-xl p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-white">Extracted Request Components</span>
                    <button
                      onClick={() => {
                        onSelectUrl(parsedCurl.url);
                        onClose();
                      }}
                      className="flex items-center gap-1.5 px-3 py-1 rounded bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-[11px] transition shadow whitespace-nowrap"
                    >
                      Load into Inspector <ArrowRight size={12} />
                    </button>
                  </div>

                  <div className="grid grid-cols-3 gap-3 font-mono text-[11px]">
                    <div className="bg-slate-900 p-2.5 rounded border border-slate-800">
                      <span className="text-slate-500 block text-[10px] uppercase">Method</span>
                      <span className="text-white font-bold">{parsedCurl.method}</span>
                    </div>
                    <div className="col-span-2 bg-slate-900 p-2.5 rounded border border-slate-800 truncate">
                      <span className="text-slate-500 block text-[10px] uppercase">Target URL</span>
                      <span className="text-indigo-400 truncate">{parsedCurl.url}</span>
                    </div>
                  </div>

                  {Object.keys(parsedCurl.headers).length > 0 && (
                    <div>
                      <span className="text-slate-400 text-[10px] font-semibold uppercase block mb-1">Headers</span>
                      <div className="bg-slate-900 rounded p-3 font-mono text-[11px] space-y-1 border border-slate-800">
                        {Object.entries(parsedCurl.headers).map(([k, v]) => (
                          <div key={k}>
                            <span className="text-slate-500">{k}:</span> <span className="text-emerald-400">{v}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center gap-4">
                <label className="flex items-center gap-2 px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-white cursor-pointer transition border border-slate-700">
                  <Upload size={14} />
                  <span>Upload .har File</span>
                  <input type="file" accept=".har,application/json" onChange={handleFileUpload} className="hidden" />
                </label>
                <span className="text-slate-500">or paste JSON content below:</span>
              </div>

              <textarea
                rows={4}
                value={harText}
                onChange={e => setHarText(e.target.value)}
                placeholder="Paste Chrome/Postman HAR export JSON..."
                className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-purple-400 font-mono text-xs focus:outline-none focus:border-purple-500"
              />

              {harError && (
                <div className="bg-rose-500/10 border border-rose-500/20 text-rose-400 p-3 rounded-lg flex items-center gap-2">
                  <AlertTriangle size={16} />
                  <span>{harError}</span>
                </div>
              )}

              <button
                onClick={() => handleParseHAR()}
                disabled={loadingHar}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-purple-600 hover:bg-purple-500 text-white font-medium transition shadow-lg shadow-purple-600/20 whitespace-nowrap"
              >
                <Play size={14} />
                {loadingHar ? 'Analyzing HAR Waterfall...' : 'Analyze HAR Waterfall'}
              </button>

              {harReport && (
                <div className="space-y-4 pt-2">
                  {/* Summary Bar */}
                  <div className="grid grid-cols-4 gap-3 font-mono text-xs">
                    <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
                      <span className="text-slate-500 text-[10px] block uppercase">Total Requests</span>
                      <span className="text-lg font-bold text-white">{harReport.totalRequests}</span>
                    </div>
                    <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
                      <span className="text-slate-500 text-[10px] block uppercase">Cacheable</span>
                      <span className="text-lg font-bold text-emerald-400">{harReport.cacheableRequests}</span>
                    </div>
                    <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
                      <span className="text-slate-500 text-[10px] block uppercase">Bypass</span>
                      <span className="text-lg font-bold text-rose-400">{harReport.bypassRequests}</span>
                    </div>
                    <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
                      <span className="text-slate-500 text-[10px] block uppercase">Cache Ratio</span>
                      <span className="text-lg font-bold text-indigo-400">{harReport.cacheabilityRatio.toFixed(1)}%</span>
                    </div>
                  </div>

                  {/* Waterfall Table */}
                  <div className="border border-slate-800 rounded-xl overflow-hidden">
                    <table className="w-full text-left text-xs">
                      <thead className="bg-slate-950 text-slate-400 border-b border-slate-800 uppercase text-[10px] tracking-wider">
                        <tr>
                          <th className="py-2 px-3">Status</th>
                          <th className="py-2 px-3">Asset URL</th>
                          <th className="py-2 px-3">MIME</th>
                          <th className="py-2 px-3">TTL</th>
                          <th className="py-2 px-3">Grade</th>
                          <th className="py-2 px-3">Blocker / Action</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800/60 font-mono text-[11px]">
                        {harReport.entries.map((entry, idx) => (
                          <tr key={idx} className="hover:bg-slate-800/30">
                            <td className="py-2.5 px-3">
                              {entry.cacheEligible ? (
                                <span className="text-emerald-400 font-sans font-semibold">CACHE</span>
                              ) : (
                                <span className="text-rose-400 font-sans font-semibold">BYPASS</span>
                              )}
                            </td>
                            <td className="py-2.5 px-3 text-slate-300 max-w-xs truncate" title={entry.url}>
                              {entry.url}
                            </td>
                            <td className="py-2.5 px-3 text-slate-500">{entry.mimeType || 'unknown'}</td>
                            <td className="py-2.5 px-3 text-indigo-400">{entry.calculatedTtl}s</td>
                            <td className="py-2.5 px-3 text-emerald-400 font-bold">{entry.auditGrade}</td>
                            <td className="py-2.5 px-3 text-slate-400 font-sans text-xs">
                              {entry.blockerReason || 'Eligible for edge cache'}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
