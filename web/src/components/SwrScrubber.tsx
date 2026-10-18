import React, { useState } from 'react';
import { Clock, Play, Zap, RefreshCw, AlertCircle, CheckCircle2 } from 'lucide-react';
import { Explanation } from '../types';

interface SwrScrubberProps {
  exp: Explanation;
}

export function SwrScrubber({ exp }: SwrScrubberProps) {
  // Extract TTL and SWR from explanation or response headers
  let ttl = exp.freshnessTtlSeconds || 0;
  let swr = 0;
  let sie = 0;

  const ccValues = exp.responseHeaders['Cache-Control'] || exp.responseHeaders['cache-control'] || [];
  const ccStr = ccValues.join(', ');

  const swrMatch = ccStr.match(/stale-while-revalidate=(\d+)/i);
  if (swrMatch) {
    swr = parseInt(swrMatch[1], 10);
  }

  const sieMatch = ccStr.match(/stale-if-error=(\d+)/i);
  if (sieMatch) {
    sie = parseInt(sieMatch[1], 10);
  }

  if (ttl === 0) {
    const maxAgeMatch = ccStr.match(/(?:s-maxage|max-age)=(\d+)/i);
    if (maxAgeMatch) {
      ttl = parseInt(maxAgeMatch[1], 10);
    } else {
      ttl = 60; // default visual baseline
    }
  }

  const maxTimeline = Math.max(ttl + swr + 60, 120);
  const [scrubTime, setScrubTime] = useState<number>(exp.currentAgeSeconds || 0);

  // Evaluate state at scrubbed time
  let stateStatus: 'FRESH' | 'SWR_STALE' | 'EXPIRED' = 'FRESH';
  let stateMessage = '';
  let badgeColor = '';

  if (scrubTime < ttl) {
    stateStatus = 'FRESH';
    stateMessage = `Response is FRESH. Served directly from edge cache in <1ms without contacting origin (${ttl - scrubTime}s freshness remaining).`;
    badgeColor = 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
  } else if (swr > 0 && scrubTime <= ttl + swr) {
    stateStatus = 'SWR_STALE';
    stateMessage = `Inside stale-while-revalidate window! Edge instantly returns cached response (0ms wait) and asynchronously dispatches background revalidation to origin.`;
    badgeColor = 'bg-cyan-500/10 text-cyan-400 border-cyan-500/20';
  } else {
    stateStatus = 'EXPIRED';
    stateMessage = `Freshness and SWR windows have expired. Client request is blocked while edge performs synchronous origin fetch or conditional 304 revalidation.`;
    badgeColor = 'bg-rose-500/10 text-rose-400 border-rose-500/20';
  }

  const freshPercent = Math.min((ttl / maxTimeline) * 100, 100);
  const swrPercent = Math.min((swr / maxTimeline) * 100, 100 - freshPercent);
  const cursorPercent = Math.min((scrubTime / maxTimeline) * 100, 100);

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Clock size={16} className="text-cyan-400" />
          <h4 className="text-xs font-semibold text-white uppercase tracking-wider">
            Interactive Cache Freshness &amp; SWR Lifecycle Scrubber
          </h4>
        </div>
        <span className={`text-[10px] font-bold px-2 py-0.5 rounded border uppercase ${badgeColor}`}>
          T+{scrubTime}s: {stateStatus}
        </span>
      </div>

      {/* Visual Timeline Bar */}
      <div className="space-y-1.5">
        <div className="relative h-4 bg-slate-950 rounded-full overflow-hidden border border-slate-800 flex">
          {/* Fresh Zone */}
          <div
            style={{ width: `${freshPercent}%` }}
            className="h-full bg-emerald-500/30 border-r border-emerald-500/40"
            title={`Fresh Lifetime: 0s to ${ttl}s`}
          />
          {/* SWR Zone */}
          {swr > 0 && (
            <div
              style={{ width: `${swrPercent}%` }}
              className="h-full bg-cyan-500/30 border-r border-cyan-500/40"
              title={`SWR Window: ${ttl}s to ${ttl + swr}s`}
            />
          )}
          {/* Expired Zone fills the rest */}
          <div className="flex-1 h-full bg-rose-500/20" title={`Expired Zone: >${ttl + swr}s`} />

          {/* Current Scrubber Indicator Pin */}
          <div
            style={{ left: `${cursorPercent}%` }}
            className="absolute top-0 bottom-0 w-1 bg-white shadow-lg shadow-white/50 -translate-x-1/2 pointer-events-none"
          />
        </div>

        {/* Legend & Labels */}
        <div className="flex items-center justify-between text-[10px] text-slate-500 font-mono">
          <span className="text-emerald-400">0s (Store)</span>
          <span className="text-emerald-400">{ttl}s (Freshness TTL)</span>
          {swr > 0 && <span className="text-cyan-400">+{swr}s (SWR End: {ttl + swr}s)</span>}
          {sie > 0 && <span className="text-purple-400">+{sie}s (SIE)</span>}
          <span className="text-rose-400">{maxTimeline}s+</span>
        </div>
      </div>

      {/* Interactive Range Slider */}
      <div className="flex items-center gap-3">
        <span className="text-[11px] text-slate-400 font-mono shrink-0">Scrub Time:</span>
        <input
          type="range"
          min={0}
          max={maxTimeline}
          value={scrubTime}
          onChange={(e) => setScrubTime(Number(e.target.value))}
          className="w-full accent-cyan-400 cursor-pointer h-1.5 bg-slate-800 rounded-lg appearance-none"
        />
        <span className="text-xs font-mono font-bold text-white shrink-0 w-12 text-right">
          {scrubTime}s
        </span>
      </div>

      {/* Dynamic Explanation Box */}
      <div className="bg-slate-950 border border-slate-800/80 rounded-lg p-3 text-xs text-slate-300 flex items-start gap-2">
        <Zap size={14} className="text-cyan-400 shrink-0 mt-0.5" />
        <span className="leading-relaxed">{stateMessage}</span>
      </div>
    </div>
  );
}
