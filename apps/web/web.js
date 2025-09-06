(function(){
  const $ = s=>document.querySelector(s);
  const SITE_URL = (() => {
    // app.xgit.ximory.com -> xgit.ximory.com
    const h = location.hostname;
    if (h.startsWith('app.')) return location.protocol + '//' + h.replace(/^app\./,'');
    // 本地/同域备用
    return '/apps/site/';
  })();

  // ===== 语言切换（仅切换已有翻译，不改变缺省中文）=====
  $('#langZh').onclick = ()=>I18N.setLang('zh');
  $('#langEn').onclick = ()=>I18N.setLang('en');

  // ===== 返回官网：同窗口跳转 =====
  $('#toSite').onclick = $('#btnBack').onclick = () => { location.href = SITE_URL; };

  // ===== Token 登录 / 刷新 / 退出 =====
  const LS_KEY = 'xgit_web_token';

  async function fetchUser(token){
    // 先 Bearer，失败再 token
    let r = await fetch('https://api.github.com/user',{
      headers:{'Authorization':'Bearer '+token,'Accept':'application/vnd.github+json'}
    });
    if (r.status === 401) {
      r = await fetch('https://api.github.com/user',{
        headers:{'Authorization':'token '+token,'Accept':'application/vnd.github+json'}
      });
    }
    if (!r.ok) throw new Error('HTTP '+r.status);
    return r.json();
  }

  async function loginFlow(){
    const tok = prompt('请输入 GitHub Token（建议 repo 权限，仅自用）：');
    if (!tok) return;
    localStorage.setItem(LS_KEY, tok);
    await refreshStatus();
  }

  async function refreshStatus(){
    const tok = localStorage.getItem(LS_KEY);
    if (!tok){
      $('#userBox').style.display='none';
      $('#btnLogin').style.display='';
      $('#btnLogin2').style.display='';
      $('#btnLogout').style.display='none';
      return;
    }
    try{
      $('#btnRefresh')?.setAttribute('disabled','disabled');
      const user = await fetchUser(tok);
      // 展示信息
      $('#userName').textContent = user.name || user.login || 'GitHub User';
      $('#userAvatar').src = (user.avatar_url || '') + '&s=80';
      $('#userBox').style.display = 'flex';
      $('#btnLogin').style.display='none';
      $('#btnLogin2').style.display='none';
      $('#btnLogout').style.display='';
    }catch(e){
      alert('登录失效或 Token 无效，请重新登录。');
      localStorage.removeItem(LS_KEY);
      $('#userBox').style.display='none';
      $('#btnLogin').style.display='';
      $('#btnLogin2').style.display='';
      $('#btnLogout').style.display='none';
    }finally{
      $('#btnRefresh')?.removeAttribute('disabled');
    }
  }

  function logout(){
    localStorage.removeItem(LS_KEY);
    refreshStatus();
  }

  $('#btnLogin').onclick = $('#btnLogin2').onclick = loginFlow;
  $('#btnRefresh').onclick = refreshStatus;
  $('#btnLogout').onclick = logout;

  // 首次进入尝试刷新状态
  document.addEventListener('DOMContentLoaded', refreshStatus);
})();