import React, { useState, useEffect } from 'react';
import { Sliders, Plus, Trash2, CheckCircle2, ShieldCheck, ToggleLeft, ToggleRight, Sparkles } from 'lucide-react';
import { EdgeRule, RuleType } from '../types';

export function RulesTab() {
  const [rules, setRules] = useState<EdgeRule[]>([]);
  const [loading, setLoading] = useState(false);
  const [showAddModal, setShowAddModal] = useState(false);

  // New rule form
  const [name, setName] = useState('');
  const [type, setType] = useState<RuleType>('strip_query_params');
  const [pathPattern, setPathPattern] = useState('.*');
  const [stripParams, setStripParams] = useState('utm_source, utm_medium, fbclid, gclid');
  const [ttlSeconds, setTtlSeconds] = useState(3600);
  const [surrogateKey, setSurrogateKey] = useState('catalog products');

  const fetchRules = async () => {
    try {
      const res = await fetch('/api/rules');
      if (res.ok) {
        setRules(await res.json());
      }
    } catch (err) {
      console.error('Error fetching rules:', err);
    }
  };

  useEffect(() => {
    fetchRules();
  }, []);

  const handleToggle = async (id: string, currentState: boolean) => {
    try {
      const res = await fetch(`/api/rules/${id}/toggle`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !currentState }),
      });
      if (res.ok) {
        setRules(await res.json());
      }
    } catch (err) {
      console.error('Error toggling rule:', err);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      const res = await fetch(`/api/rules/${id}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        setRules(await res.json());
      }
    } catch (err) {
      console.error('Error deleting rule:', err);
    }
  };

  const handleAddRule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    setLoading(true);
    try {
      const newRule: Partial<EdgeRule> = {
        name,
        type,
        enabled: true,
        pathPattern,
      };

      if (type === 'strip_query_params') {
        newRule.stripParams = stripParams.split(',').map(s => s.trim()).filter(Boolean);
      } else if (type === 'override_ttl') {
        newRule.ttlSeconds = Number(ttlSeconds);
      } else if (type === 'add_surrogate_keys') {
        newRule.surrogateKey = surrogateKey;
      }

      const res = await fetch('/api/rules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newRule),
      });

      if (res.ok) {
        setRules(await res.json());
        setShowAddModal(false);
        setName('');
      }
    } catch (err) {
      console.error('Error adding rule:', err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-teal-500/10 border border-teal-500/20 text-teal-400">
              <Sliders size={20} />
            </div>
            <div>
              <h2 className="text-base font-semibold text-white">Edge Transform & Normalization Rules</h2>
              <p className="text-xs text-slate-400">
                Simulate Cloudflare Page Rules, Fastly VCL snippet transforms, and CloudFront Functions directly at the edge proxy.
              </p>
            </div>
          </div>
          <button
            onClick={() => setShowAddModal(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-teal-600 hover:bg-teal-500 text-white transition shadow-lg shadow-teal-600/20 shrink-0 whitespace-nowrap"
          >
            <Plus size={14} />
            Add Rule
          </button>
        </div>

        {/* Rules Table */}
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-950 text-slate-400 border-b border-slate-800 uppercase text-[10px] tracking-wider">
              <tr>
                <th className="py-2.5 px-3">Status</th>
                <th className="py-2.5 px-3">Rule Name</th>
                <th className="py-2.5 px-3">Type</th>
                <th className="py-2.5 px-3">Path Pattern</th>
                <th className="py-2.5 px-3">Details</th>
                <th className="py-2.5 px-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-mono text-[11px]">
              {rules.map(rule => (
                <tr key={rule.id} className="hover:bg-slate-800/30">
                  <td className="py-3 px-3">
                    <button
                      onClick={() => handleToggle(rule.id, rule.enabled)}
                      className="text-slate-400 hover:text-white transition"
                    >
                      {rule.enabled ? (
                        <ToggleRight size={22} className="text-emerald-400" />
                      ) : (
                        <ToggleLeft size={22} className="text-slate-600" />
                      )}
                    </button>
                  </td>
                  <td className="py-3 px-3 font-sans font-semibold text-white">
                    {rule.name}
                  </td>
                  <td className="py-3 px-3">
                    <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-slate-800 text-teal-400 border border-slate-700">
                      {rule.type}
                    </span>
                  </td>
                  <td className="py-3 px-3 text-slate-300">
                    <code>{rule.pathPattern}</code>
                  </td>
                  <td className="py-3 px-3 text-slate-400 font-sans text-xs">
                    {rule.type === 'strip_query_params' && (
                      <span>Strip params: <code className="text-emerald-400">{rule.stripParams?.join(', ') || 'all'}</code></span>
                    )}
                    {rule.type === 'strip_cookies' && <span>Strips Request & Response Cookies on matched assets</span>}
                    {rule.type === 'override_ttl' && (
                      <span>Forces Edge TTL to <code className="text-indigo-400">{rule.ttlSeconds}s</code></span>
                    )}
                    {rule.type === 'add_surrogate_keys' && (
                      <span>Injects tags: <code className="text-amber-400">{rule.surrogateKey}</code></span>
                    )}
                    {rule.type === 'bypass_cache' && <span>Unconditionally bypasses cache to origin</span>}
                  </td>
                  <td className="py-3 px-3 text-right">
                    <button
                      onClick={() => handleDelete(rule.id)}
                      className="p-1.5 rounded hover:bg-rose-500/20 text-slate-400 hover:text-rose-400 transition"
                      title="Delete Rule"
                    >
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Add Rule Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-lg w-full p-6 shadow-2xl space-y-4">
            <h3 className="text-base font-semibold text-white">Add New Edge Transform Rule</h3>

            <form onSubmit={handleAddRule} className="space-y-4 text-xs">
              <div>
                <label className="block text-slate-400 mb-1 font-semibold uppercase text-[10px]">Rule Name</label>
                <input
                  type="text"
                  required
                  value={name}
                  onChange={e => setName(e.target.value)}
                  placeholder="e.g. Strip Facebook & Google Tracking Params"
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-teal-500 font-sans"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-slate-400 mb-1 font-semibold uppercase text-[10px]">Rule Type</label>
                  <select
                    value={type}
                    onChange={e => setType(e.target.value as RuleType)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-teal-500 font-mono"
                  >
                    <option value="strip_query_params">Strip Query Params</option>
                    <option value="strip_cookies">Strip Cookies</option>
                    <option value="override_ttl">Override TTL</option>
                    <option value="add_surrogate_keys">Inject Surrogate Keys</option>
                    <option value="bypass_cache">Bypass Cache</option>
                  </select>
                </div>
                <div>
                  <label className="block text-slate-400 mb-1 font-semibold uppercase text-[10px]">Path Pattern (Regex/Glob)</label>
                  <input
                    type="text"
                    required
                    value={pathPattern}
                    onChange={e => setPathPattern(e.target.value)}
                    placeholder=".*"
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-teal-500 font-mono"
                  />
                </div>
              </div>

              {type === 'strip_query_params' && (
                <div>
                  <label className="block text-slate-400 mb-1 font-semibold uppercase text-[10px]">Comma-Separated Params</label>
                  <input
                    type="text"
                    value={stripParams}
                    onChange={e => setStripParams(e.target.value)}
                    placeholder="utm_source, utm_medium, fbclid, gclid"
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-teal-500 font-mono"
                  />
                </div>
              )}

              {type === 'override_ttl' && (
                <div>
                  <label className="block text-slate-400 mb-1 font-semibold uppercase text-[10px]">TTL in Seconds</label>
                  <input
                    type="number"
                    value={ttlSeconds}
                    onChange={e => setTtlSeconds(Number(e.target.value))}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-teal-500 font-mono"
                  />
                </div>
              )}

              {type === 'add_surrogate_keys' && (
                <div>
                  <label className="block text-slate-400 mb-1 font-semibold uppercase text-[10px]">Surrogate Keys (Space-Separated)</label>
                  <input
                    type="text"
                    value={surrogateKey}
                    onChange={e => setSurrogateKey(e.target.value)}
                    placeholder="catalog products v1"
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-teal-500 font-mono"
                  />
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition whitespace-nowrap"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={loading}
                  className="px-4 py-2 rounded-lg bg-teal-600 hover:bg-teal-500 text-white font-medium transition shadow-lg shadow-teal-600/20 disabled:opacity-50 whitespace-nowrap"
                >
                  {loading ? 'Adding...' : 'Create Edge Rule'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
