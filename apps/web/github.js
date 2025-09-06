// Minimal GitHub REST API wrapper (browser fetch)
const GH = 'https://api.github.com';

async function ghFetch(token, url, opts={}) {
  const res = await fetch(url, {
    ...opts,
    headers: {
      'Accept': 'application/vnd.github+json',
      'X-GitHub-Api-Version': '2022-11-28',
      ...(opts.headers||{}),
      ...(token ? { 'Authorization': `token ${token}` } : {})
    }
  });
  if (!res.ok) {
    const text = await res.text().catch(()=> '');
    throw new Error(`${res.status} ${res.statusText} ${text}`.trim());
  }
  const ct = res.headers.get('content-type') || '';
  if (ct.includes('application/json')) return res.json();
  return res.text();
}

export const api = {
  me(token) {
    return ghFetch(token, `${GH}/user`);
  },

  async repos(token, visibility='all') {
    // 简化：取第一页 100 条
    const url = `${GH}/user/repos?per_page=100&sort=updated`;
    return ghFetch(token, url);
  },

  listPath(token, {owner, repo, path='', ref}) {
    const p = encodeURIComponent(path);
    const q = ref ? `?ref=${encodeURIComponent(ref)}` : '';
    return ghFetch(token, `${GH}/repos/${owner}/${repo}/contents/${p}${q}`);
  },

  getFile(token, {owner, repo, path, ref}) {
    const p = encodeURIComponent(path);
    const q = ref ? `?ref=${encodeURIComponent(ref)}` : '';
    return ghFetch(token, `${GH}/repos/${owner}/${repo}/contents/${p}${q}`);
  },

  putFile(token, {owner, repo, path, message, contentBase64, sha, branch}) {
    const p = encodeURIComponent(path);
    const body = {
      message,
      content: contentBase64,
      ...(sha ? { sha } : {}),
      ...(branch ? { branch } : {})
    };
    return ghFetch(token, `${GH}/repos/${owner}/${repo}/contents/${p}`, {
      method: 'PUT',
      body: JSON.stringify(body)
    });
  },

  commits(token, {owner, repo, path, sha}) {
    const qs = new URLSearchParams();
    if (path) qs.set('path', path);
    if (sha) qs.set('sha', sha);
    const q = qs.toString() ? `?${qs.toString()}` : '';
    return ghFetch(token, `${GH}/repos/${owner}/${repo}/commits${q}`);
  }
};