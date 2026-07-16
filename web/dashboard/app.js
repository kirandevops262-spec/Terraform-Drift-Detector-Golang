const API = '/api/v1';

let selectedScanId = null;

async function fetchJSON(url, options = {}) {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function setStatus(msg) {
  document.getElementById('status').textContent = msg;
}

function formatTime(iso) {
  if (!iso) return '-';
  return new Date(iso).toLocaleString();
}

function driftBadge(type) {
  return `<span class="drift-type drift-${type}">${type}</span>`;
}

async function loadScans() {
  setStatus('Loading...');
  try {
    const scans = await fetchJSON(`${API}/scans`);
    const tbody = document.querySelector('#scans-table tbody');
    tbody.innerHTML = '';

    document.getElementById('scan-count').textContent = scans.length;

    if (scans.length > 0) {
      const latest = scans[0];
      document.getElementById('last-scan').textContent = formatTime(latest.started_at);
      document.getElementById('drift-count').textContent =
        latest.report ? latest.report.summary.total : '-';
    }

    for (const scan of scans) {
      const tr = document.createElement('tr');
      const drifts = scan.report ? scan.report.summary.total : '-';
      tr.innerHTML = `
        <td><code>${scan.id.slice(0, 8)}...</code></td>
        <td><span class="badge ${scan.status}">${scan.status}</span></td>
        <td>${formatTime(scan.started_at)}</td>
        <td>${drifts}</td>
        <td>
          ${scan.report ? `<button class="link-btn" data-id="${scan.id}">View</button>` : ''}
        </td>
      `;
      tbody.appendChild(tr);
    }

    tbody.querySelectorAll('.link-btn').forEach(btn => {
      btn.addEventListener('click', () => showDrifts(btn.dataset.id));
    });

    setStatus('');
  } catch (err) {
    setStatus('Error: ' + err.message);
  }
}

async function showDrifts(scanId) {
  selectedScanId = scanId;
  document.getElementById('detail-section').classList.remove('hidden');
  document.getElementById('detail-scan-id').textContent = `(${scanId.slice(0, 8)}...)`;

  const drifts = await fetchJSON(`${API}/scans/${scanId}/drifts`);
  const tbody = document.querySelector('#drifts-table tbody');
  tbody.innerHTML = '';

  for (const d of drifts) {
    const tr = document.createElement('tr');
    const ref = d.terraform_ref || d.cloud_id || d.resource_id;
    let details = d.message || '';
    if (d.changes && d.changes.length) {
      details = d.changes.map(c => `${c.path}: ${JSON.stringify(c.expected)} → ${JSON.stringify(c.actual)}`).join('; ');
    }
    tr.innerHTML = `
      <td>${d.type}</td>
      <td><code>${ref}</code></td>
      <td>${driftBadge(d.drift_type)}</td>
      <td>${details}</td>
    `;
    tbody.appendChild(tr);
  }
}

async function runScan() {
  const btn = document.getElementById('run-scan');
  btn.disabled = true;
  setStatus('Running scan...');
  try {
    await fetchJSON(`${API}/scans`, { method: 'POST' });
    await loadScans();
    setStatus('Scan completed');
  } catch (err) {
    setStatus('Scan failed: ' + err.message);
  } finally {
    btn.disabled = false;
  }
}

async function exportJSON() {
  if (!selectedScanId) return;
  const report = await fetchJSON(`${API}/scans/${selectedScanId}/report`);
  const blob = new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `drift-report-${selectedScanId.slice(0, 8)}.json`;
  a.click();
}

document.getElementById('run-scan').addEventListener('click', runScan);
document.getElementById('refresh').addEventListener('click', loadScans);
document.getElementById('export-json').addEventListener('click', exportJSON);

loadScans();
