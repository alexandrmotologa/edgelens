import { Explanation } from '../types';

export function exportDiagnosticHTML(exp: Explanation) {
  const htmlContent = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>EdgeLens RFC 9111 Diagnostic Report - ${exp.id}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; padding: 32px; line-height: 1.5; }
    .container { max-width: 900px; margin: 0 auto; background: #1e293b; border: 1px solid #334155; border-radius: 12px; padding: 24px; }
    h1 { font-size: 20px; margin-top: 0; color: #38bdf8; }
    .badge { display: inline-block; padding: 4px 10px; border-radius: 6px; font-weight: bold; font-size: 12px; text-transform: uppercase; }
    .badge-HIT { background: rgba(16, 185, 129, 0.2); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.4); }
    .badge-MISS { background: rgba(245, 158, 11, 0.2); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.4); }
    .badge-STALE { background: rgba(6, 182, 212, 0.2); color: #22d3ee; border: 1px solid rgba(6, 182, 212, 0.4); }
    .badge-BYPASS { background: rgba(168, 85, 247, 0.2); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.4); }
    .meta-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin: 20px 0; font-family: monospace; font-size: 12px; }
    .meta-card { background: #0f172a; padding: 12px; border-radius: 8px; border: 1px solid #334155; }
    .meta-title { color: #94a3b8; font-size: 10px; text-transform: uppercase; margin-bottom: 4px; }
    .phase { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 14px; margin-bottom: 12px; }
    .phase-title { font-weight: bold; font-size: 13px; margin-bottom: 6px; color: #f1f5f9; display: flex; align-items: center; gap: 8px; }
    .passed { color: #34d399; }
    .failed { color: #f87171; }
    .code-box { background: #020617; padding: 12px; border-radius: 6px; font-family: monospace; font-size: 11px; color: #a5f3fc; overflow-x: auto; margin-top: 6px; }
    ul { margin: 6px 0; padding-left: 20px; font-size: 12px; color: #cbd5e1; }
  </style>
</head>
<body>
  <div class="container">
    <h1>⚡ EdgeLens RFC 9111 Diagnostic Report</h1>
    <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px;">
      <div>
        <span class="badge badge-${exp.finalStatus}">${exp.finalStatus}</span>
        <strong style="margin-left: 12px; font-family: monospace;">${exp.method} ${exp.url}</strong>
      </div>
      <div style="font-size: 12px; color: #94a3b8;">${new Date(exp.timestamp).toLocaleString()}</div>
    </div>

    <div class="meta-grid">
      <div class="meta-card">
        <div class="meta-title">Total Latency</div>
        <div><strong>${exp.latencyMs}ms</strong> (Origin: ${exp.originLatencyMs}ms)</div>
      </div>
      <div class="meta-card">
        <div class="meta-title">Active Flavor</div>
        <div style="text-transform: capitalize;"><strong>${exp.cdnFlavor}</strong></div>
      </div>
      <div class="meta-card">
        <div class="meta-title">Freshness TTL</div>
        <div><strong>${exp.freshnessTtlSeconds}s</strong> (Age: ${exp.currentAgeSeconds}s)</div>
      </div>
    </div>

    <h3>5-Phase RFC 9111 Decision Explainer</h3>
    ${exp.phases.map(p => `
      <div class="phase">
        <div class="phase-title">
          <span class="${p.passed ? 'passed' : 'failed'}">${p.passed ? '✓' : '✗'}</span>
          <span>${p.name}</span>
        </div>
        <div style="font-size: 12px; color: #94a3b8;">${p.description}</div>
        <ul>
          ${p.details.map(d => `<li>${d}</li>`).join('')}
        </ul>
      </div>
    `).join('')}

    <h3>Response Headers</h3>
    <div class="code-box">
${Object.entries(exp.responseHeaders).map(([k, vv]) => `${k}: ${vv.join(', ')}`).join('\n')}
    </div>
  </div>
</body>
</html>`;

  const blob = new Blob([htmlContent], { type: 'text/html' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `edgelens-report-${exp.id}.html`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export function generateDiagnosticMarkdown(exp: Explanation): string {
  let md = `## ⚡ EdgeLens RFC 9111 Diagnostic Report\n\n`;
  md += `**Target:** \`${exp.method} ${exp.url}\`\n`;
  md += `**Status:** \`${exp.finalStatus}\` | **Latency:** \`${exp.latencyMs}ms\` | **CDN Flavor:** \`${exp.cdnFlavor}\`\n`;
  md += `**Freshness TTL:** \`${exp.freshnessTtlSeconds}s\` (Current Age: \`${exp.currentAgeSeconds}s\`)\n\n`;

  md += `### 5-Phase RFC 9111 Decision Trace\n\n`;
  exp.phases.forEach((p, idx) => {
    const icon = p.passed ? '✓' : '✗';
    md += `**${idx + 1}. [${icon}] ${p.name}**\n`;
    md += `_${p.description}_\n`;
    p.details.forEach(d => {
      md += `- ${d}\n`;
    });
    md += `\n`;
  });

  if (exp.recommendations && exp.recommendations.length > 0) {
    md += `### Recommendations\n\n`;
    exp.recommendations.forEach(r => {
      md += `- **[${r.level.toUpperCase()}] ${r.title}:** ${r.message}\n`;
    });
    md += `\n`;
  }

  md += `### Response Headers\n\n\`\`\`http\n`;
  Object.entries(exp.responseHeaders).forEach(([k, vv]) => {
    md += `${k}: ${vv.join(', ')}\n`;
  });
  md += `\`\`\`\n`;

  return md;
}
