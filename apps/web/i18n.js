// i18n 底座：有翻译就替换；没有就保留页面里原本中文（不改动）
const I18N = (() => {
  const STORAGE_KEY = 'xgit_web_lang';
  let lang = localStorage.getItem(STORAGE_KEY) || 'zh';

  // 只需要补“有差异”的英文；其它缺失会保留原文（中文）
  const dict = {
    zh: {
      loginByToken: '用 Token 登录',
      logout: '退出登录',
      welcome: '欢迎使用 XGit Web',
      tagline: '随时随地，轻松管理你的 GitHub 仓库。',
      backToSite: '返回官网',
      refresh: '刷新',
      loginOk: '已登录'
    },
    en: {
      loginByToken: 'Sign in with Token',
      logout: 'Sign out',
      welcome: 'Welcome to XGit Web',
      tagline: 'Manage your GitHub repos on the go.',
      backToSite: 'Back to Site',
      refresh: 'Refresh',
      loginOk: 'Signed in'
    }
  };

  function apply(container=document){
    const nodes = container.querySelectorAll('[data-i18n]');
    nodes.forEach(el=>{
      const key = el.getAttribute('data-i18n');
      const trans = dict[lang]?.[key];
      if (trans) el.textContent = trans; // 只有有翻译时才替换；否则保留原本中文
    });
  }

  function setLang(next){
    if (!['zh','en'].includes(next)) return;
    lang = next;
    localStorage.setItem(STORAGE_KEY, lang);
    apply(document);
  }

  // URL ?lang= 覆盖
  const urlLang = new URLSearchParams(location.search).get('lang');
  if (urlLang) setLang(urlLang);

  // 首次渲染
  document.addEventListener('DOMContentLoaded', ()=>apply(document));

  return { setLang, getLang:()=>lang };
})();