/**
 * gen-bindings.js — 把 wails 原生生成的 App.js 改写为双模式桥接版
 *
 * 背景：
 *   `wails build` 每次都会重新生成 frontend/wailsjs/go/main/App.js，内容形如
 *       export function Foo(arg1) {
 *         return window['go']['main']['App']['Foo'](arg1);
 *       }
 *   这种写法只在 Wails 桌面容器里有效；Web 模式（-tags web）下 window['go'] 不存在，
 *   全部绑定调用会直接抛错。
 *
 *   而本项目要求同一份前端代码同时跑在桌面与浏览器两种形态下，因此必须让每个绑定
 *   都走 bridge.js 的 call()，由它按运行环境分发。
 *
 * 做法：
 *   本脚本在 `npm run build` 中最先执行（wails build 会调用 frontend:build，
 *   且顺序上在绑定生成之后），把原生版改写为桥接版。因为是每次构建都跑，
 *   所以无论 wails 刚才生成了什么，最终进入 vite 的一定是桥接版 —— 自愈，
 *   不需要开发者记得加 -skipbindings，也不会再出现"手改文件被静默覆盖"的事故。
 *
 * 安全性：
 *   改写完成后会断言"导出函数数 == 走 call() 的函数数"。一旦 wails 改变了生成格式
 *   导致正则失配，这里会直接报错退出，而不是安静地产出一个 Web 模式不可用的包。
 */

const fs = require('fs');
const path = require('path');

const target = path.join(__dirname, '..', 'wailsjs', 'go', 'main', 'App.js');

const HEADER = [
  '// @ts-check',
  '// AI盘译 前端绑定层（双模式）—— 由 scripts/gen-bindings.js 自动生成，请勿手工编辑。',
  '//',
  '// wails 原生生成的结果是 window[\'go\'] 直连，只在桌面容器里有效；这里统一改写为',
  '// 走 bridge.js 的 call()，由它按运行环境分发：',
  '//   - 桌面模式（Wails）：转发到 window[\'go\'][\'main\'][\'App\'][method]',
  '//   - Web 模式（浏览器）：POST /api/call 反射派发',
  '// 业务代码 import 方式不变。',
  '',
  "import { call } from '../../bridge.js';",
  '',
].join('\n');

// 原生格式恒定是三行一组，这里按"函数签名行 + return 行 + 右花括号"成组匹配。
const NATIVE = /export function (\w+)\(([^)]*)\) \{\r?\n\s*return window\['go'\]\['main'\]\['App'\]\['\1'\]\([^)]*\);\r?\n\}/g;

function main() {
  if (!fs.existsSync(target)) {
    console.error('[gen-bindings] 找不到 ' + target);
    process.exit(1);
  }

  let src = fs.readFileSync(target, 'utf8');
  const crlf = src.includes('\r\n');
  src = src.replace(/\r\n/g, '\n');

  // 已经是桥接版就直接结束，保证幂等（重复执行 npm run build 不会反复改写）
  if (src.includes("from '../../bridge.js'")) {
    console.log('[gen-bindings] App.js 已是桥接版，跳过');
    return;
  }

  // 剥掉 wails 的头部注释（// @ts-check + DO NOT EDIT 等），避免与下面的 HEADER 重复
  src = src.replace(/^(?:\/\/[^\n]*\n)+/, '').replace(/^\n+/, '');

  const names = [];
  const out = src.replace(NATIVE, (_m, name) => {
    names.push(name);
    return 'export function ' + name + '(...args) {\n  return call(\'' + name + '\', args);\n}';
  });

  // 失配保护：原生文件里所有 export function 都必须被改写
  const total = (src.match(/^export function /gm) || []).length;
  if (names.length !== total) {
    console.error(
      '[gen-bindings] 改写不完整：匹配到 ' + names.length + ' / ' + total +
      '。wails 的生成格式可能已变化，请检查 gen-bindings.js 中的 NATIVE 正则。'
    );
    process.exit(1);
  }

  let result = HEADER + out + '\n';
  if (crlf) result = result.replace(/\n/g, '\r\n');
  fs.writeFileSync(target, result);
  console.log('[gen-bindings] App.js 已改写为桥接版，共 ' + names.length + ' 个绑定');
}

main();