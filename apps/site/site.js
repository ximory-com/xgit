(function(){
  const KEY='xgit_site_lang';
  let lang = localStorage.getItem(KEY) || ((navigator.language||'').toLowerCase().startsWith('zh')?'zh':'en');
  const dict = {
    zh:{
      openApp:'打开 Web App', viewRepo:'查看仓库',
      headline:'更顺手的 Git 编辑器（移动端 & 网页端）',
      subhead:'聚焦仓库浏览、编辑、Diff、多文件一次提交。无需服务端，直接使用 GitHub API。',
      features:'特性', compare:'对比', changelog:'更新',
      bullets:[ '移动端优先，编辑器可全屏', '多文件勾选后一次提交', '支持空目录（.keep）与新建文件（规划中）' ],
      compareHead:['功能','XGit','GitHub App'],
      compareRows:[ ['多文件一次提交','✅','❌'], ['移动端编辑体验','✅','一般'], ['无需后端','✅','✅'] ],
      changes:[
        {ver:'v0.1.0',date:'2025-09-06',desc:'初始化目录、官网与 Web App 起步版本。'},
        {ver:'v0.1.0',date:'2025-09-06',desc:'统一 logo 路径 ./assets/logo.svg。'},
        {ver:'v0.1.0',date:'2025-09-06',desc:'站点与 App 的多语言各自独立。'}
      ]
    },
    en:{
      openApp:'Open Web App', viewRepo:'View Repo',
      headline:'A handier Git editor (mobile & web)',
      subhead:'Focus on browse, edit, diff, multi-file commit. No server needed (GitHub API only).',
      features:'Features', compare:'Comparison', changelog:'Changelog',
      bullets:[ 'Mobile-first, fullscreen editor', 'Commit multiple files at once', 'Empty dirs (.keep) & new file (planned)' ],
      compareHead:['Feature','XGit','GitHub App'],
      compareRows:[ ['Multi-file commit','✅','❌'], ['Mobile editing UX','✅','Fair'], ['No backend','✅','✅'] ],
      changes:[
        {ver:'v0.1.0',date:'2025-09-06',desc:'Initialize structure, site and web starters.'},
        {ver:'v0.1.0',date:'2025-09-06',desc:'Unify logo path ./assets/logo.svg.'},
        {ver:'v0.1.0',date:'2025-09-06',desc:'Separate i18n for site and app.'}
      ]
    }
  };
  const $ = s=>document.querySelector(s);
  function apply(){
    document.querySelectorAll('[data-i18n]').forEach(el=>{
      const k=el.dataset.i18n; el.textContent=(dict[lang]&&dict[lang][k])||k;
    });
    const bullets = dict[lang].bullets||[];
    $('#feat').innerHTML = bullets.map(b=>`<li>${b}</li>`).join('');
    const head = dict[lang].compareHead||[];
    $('#cmpHead').innerHTML = head.map(h=>`<th>${h}</th>`).join('');
    const rows = dict[lang].compareRows||[];
    $('#cmpBody').innerHTML = rows.map(r=>`<tr>${r.map(c=>`<td>${c}</td>`).join('')}</tr>`).join('');
    const changes = dict[lang].changes||[];
    $('#changes').innerHTML = changes.map(it=>`<div class="item"><b>${it.ver}</b> • ${it.date}<div>${it.desc}</div></div>`).join('');
  }
  function setLang(l){ lang=l; localStorage.setItem(KEY,l); apply(); }
  document.addEventListener('DOMContentLoaded', ()=>{
    document.getElementById('btnZh').onclick=()=>setLang('zh');
    document.getElementById('btnEn').onclick=()=>setLang('en');
    apply();
  });
})();