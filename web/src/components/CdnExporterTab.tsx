import React, { useState, useEffect } from 'react';
import { Download, Copy, Check, FileCode, CheckCircle2, Sliders, Globe } from 'lucide-react';
import { GeneratedConfigs } from '../types';

export function CdnExporterTab() {
  const [backendHost, setBackendHost] = useState('origin.example.com');
  const [pathPrefix, setPathPrefix] = useState('/api/');
  const [edgeTtl, setEdgeTtl] = useState(3600);
  const [browserTtl, setBrowserTtl] = useState(300);
  const [enableSwr, setEnableSwr] = useState(true);
  const [swrDuration, setSwrDuration] = useState(86400);
  const [enableSie, setEnableSie] = useState(true);
  const [sieDuration, setSieDuration] = useState(86400);
  const [surrogateKeys, setSurrogateKeys] = useState('catalog, products, v1');
  const [stripCookies, setStripCookies] = useState(true);
  const [stripMarketingQp, setStripMarketingQp] = useState(true);

  const [activePlatform, setActivePlatform] = useState<'nginx' | 'fastly' | 'cloudflare' | 'cloudfront'>('nginx');
  const [configs, setConfigs] = useState<GeneratedConfigs | null>(null);
  const [copied, setCopied] = useState(false);

  const fetchConfigs = async () => {
    try {
      const res = await fetch('/api/export-cdn', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          backendHost,
          pathPrefix,
          edgeTtlSeconds: Number(edgeTtl),
          browserTtlSeconds: Number(browserTtl),
          enableSwr,
          swrDurationSeconds: Number(swrDuration),
          enableSie,
          sieDurationSeconds: Number(sieDuration),
          surrogateKeys: surrogateKeys.split(',').map(s => s.trim()).filter(Boolean),
          stripCookies,
          stripMarketingQp,
        }),
      });

      if (res.ok) {
        setConfigs(await res.json());
      }
    } catch (err) {
      console.error('Error exporting CDN config:', err);
    }
  };

  useEffect(() => {
    fetchConfigs();
  }, [backendHost, pathPrefix, edgeTtl, browserTtl, enableSwr, swrDuration, enableSie, sieDuration, surrogateKeys, stripCookies, stripMarketingQp]);

  const getActiveCode = () => {
    if (!configs) return '';
    switch (activePlatform) {
      case 'nginx':
        return configs.nginxConf;
      case 'fastly':
        return configs.fastlyVcl;
      case 'cloudflare':
        return configs.cloudflareTerraform;
      case 'cloudfront':
        return configs.cloudFrontTerraform;
    }
  };

  const handleCopy = () => {
    const code = getActiveCode();
    if (!code) return;
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="space-y-6">
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
            <Download size={20} />
          </div>
          <div>
            <h2 className="text-base font-semibold text-white">Production CDN & Reverse Proxy Config Exporter</h2>
            <p className="text-xs text-slate-400">
              Transform your validated EdgeLens caching rules into ready-to-deploy configuration files for Nginx, Fastly VCL, Cloudflare Terraform, and AWS CloudFront.
            </p>
          </div>
        </div>

        {/* Inputs */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-3 text-xs mb-4">
          <div>
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Backend Host</label>
            <input
              type="text"
              value={backendHost}
              onChange={e => setBackendHost(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-emerald-500"
            />
          </div>
          <div>
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Path Prefix</label>
            <input
              type="text"
              value={pathPrefix}
              onChange={e => setPathPrefix(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-emerald-500"
            />
          </div>
          <div>
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Edge TTL (s)</label>
            <input
              type="number"
              value={edgeTtl}
              onChange={e => setEdgeTtl(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-emerald-500"
            />
          </div>
          <div>
            <label className="block text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-1">Browser TTL (s)</label>
            <input
              type="number"
              value={browserTtl}
              onChange={e => setBrowserTtl(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-emerald-500"
            />
          </div>
        </div>

        {/* Toggles */}
        <div className="flex flex-wrap items-center gap-6 text-xs text-slate-300 pt-2 border-t border-slate-800/80">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={enableSwr}
              onChange={e => setEnableSwr(e.target.checked)}
              className="rounded bg-slate-950 border-slate-800 text-emerald-500 focus:ring-0"
            />
            <span>Stale-While-Revalidate (SWR)</span>
          </label>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={enableSie}
              onChange={e => setEnableSie(e.target.checked)}
              className="rounded bg-slate-950 border-slate-800 text-emerald-500 focus:ring-0"
            />
            <span>Stale-If-Error Failover (SIE)</span>
          </label>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={stripCookies}
              onChange={e => setStripCookies(e.target.checked)}
              className="rounded bg-slate-950 border-slate-800 text-emerald-500 focus:ring-0"
            />
            <span>Strip Set-Cookie on HIT</span>
          </label>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={stripMarketingQp}
              onChange={e => setStripMarketingQp(e.target.checked)}
              className="rounded bg-slate-950 border-slate-800 text-emerald-500 focus:ring-0"
            />
            <span>Strip Marketing Query Params</span>
          </label>
        </div>
      </div>

      {/* Code Viewer */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
        <div className="flex items-center justify-between border-b border-slate-800 bg-slate-950 px-4 py-2.5">
          <div className="flex items-center gap-1">
            <button
              onClick={() => setActivePlatform('nginx')}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                activePlatform === 'nginx' ? 'bg-slate-800 text-white font-semibold' : 'text-slate-400 hover:text-white'
              }`}
            >
              Nginx Cache
            </button>
            <button
              onClick={() => setActivePlatform('fastly')}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                activePlatform === 'fastly' ? 'bg-slate-800 text-white font-semibold' : 'text-slate-400 hover:text-white'
              }`}
            >
              Fastly VCL
            </button>
            <button
              onClick={() => setActivePlatform('cloudflare')}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                activePlatform === 'cloudflare' ? 'bg-slate-800 text-white font-semibold' : 'text-slate-400 hover:text-white'
              }`}
            >
              Cloudflare (Terraform)
            </button>
            <button
              onClick={() => setActivePlatform('cloudfront')}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                activePlatform === 'cloudfront' ? 'bg-slate-800 text-white font-semibold' : 'text-slate-400 hover:text-white'
              }`}
            >
              AWS CloudFront (Terraform)
            </button>
          </div>

          <button
            onClick={handleCopy}
            className="flex items-center gap-1.5 px-3 py-1 text-xs font-medium rounded-md bg-emerald-600 hover:bg-emerald-500 text-white transition shadow"
          >
            {copied ? <Check size={14} /> : <Copy size={14} />}
            {copied ? 'Copied Snippet' : 'Copy Code'}
          </button>
        </div>

        <div className="p-5 bg-slate-950 overflow-x-auto">
          <pre className="font-mono text-xs text-emerald-400 leading-relaxed">
            {getActiveCode()}
          </pre>
        </div>
      </div>
    </div>
  );
}
