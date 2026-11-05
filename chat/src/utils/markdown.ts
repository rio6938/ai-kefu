/**
 * 轻量 Markdown 渲染：聊天气泡只需要标题、列表、引用、代码块、行内样式与链接，
 * 因此不引入额外依赖，渲染前先转义 HTML，避免消息内容里的标签被当成 DOM 执行
 */

/** 行内片段占位符，已渲染的片段先存起来，避免被后续规则重复匹配 */
const tokenStart = '\u0001';
const tokenEnd = '\u0002';

/** 允许渲染的链接协议，其余一律不生成 a / img 标签 */
const allowedProtocols = ['http://', 'https://', 'mailto:', 'tel:'];

/** 代码块起始行 */
const fencePattern = /^\s*```\s*([\w+-]*)\s*$/;

/** 代码块结束行 */
const fenceEndPattern = /^\s*```\s*$/;

/** 块级语法：标题、分隔线、引用、列表项 */
const headingPattern = /^(#{1,6})\s+(.*)$/;

/** 分隔线 */
const dividerPattern = /^([-*_])\s*(\1\s*){2,}$/;

/** 引用 */
const quotePattern = /^>\s?/;

/** 表格分隔行，如 | --- | :---: | */
const tableDividerPattern = /^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$/;

/** 列表项 */
const listItemPattern = /^(\s*)([-*+]|\d+[.)])\s+(.*)$/;

/**
 * 转义 HTML 特殊字符
 * @param text 原始文本
 */
function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/**
 * 校验链接地址，只放行常见协议
 * @param url 原始地址
 */
function safeUrl(url: string): string {
  const value = url.trim();
  const lower = value.toLowerCase();
  return allowedProtocols.some((prefix) => lower.startsWith(prefix))
    ? value
    : '';
}

/**
 * 判断是否是块级语法的起始行
 * @param line 行内容
 */
function isBlockStart(line: string): boolean {
  return (
    fencePattern.test(line) ||
    fenceEndPattern.test(line) ||
    headingPattern.test(line) ||
    dividerPattern.test(line) ||
    quotePattern.test(line) ||
    listItemPattern.test(line)
  );
}

/**
 * 渲染行内样式：行内代码、图片、链接、加粗、删除线、斜体、裸链接
 * @param text 原始文本
 */
function renderInline(text: string): string {
  const tokens: string[] = [];

  /** 暂存一段已渲染的 HTML，返回占位符 */
  const stash = (html: string) => {
    tokens.push(html);
    return `${tokenStart}${tokens.length - 1}${tokenEnd}`;
  };

  // 先转义，后续只插入本文件生成的标签
  let result = escapeHtml(text);
  // 行内代码优先，避免代码里的星号被当成加粗
  result = result.replace(/`([^`\n]+)`/g, (_match, code: string) =>
    stash(`<code>${code}</code>`)
  );
  // 图片
  result = result.replace(
    /!\[([^\]]*)\]\(([^)\s]+)\)/g,
    (match, alt: string, url: string) => {
      const src = safeUrl(url);
      return src ? stash(`<img src="${src}" alt="${alt}" />`) : match;
    }
  );
  // 链接
  result = result.replace(
    /\[([^\]]*)\]\(([^)\s]+)\)/g,
    (match, label: string, url: string) => {
      const href = safeUrl(url);
      return href
        ? stash(
            `<a href="${href}" target="_blank" rel="noopener noreferrer">${label}</a>`
          )
        : match;
    }
  );
  // 加粗、删除线、斜体
  result = result.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
  result = result.replace(/__([^_\n]+)__/g, '<strong>$1</strong>');
  result = result.replace(/~~([^~\n]+)~~/g, '<del>$1</del>');
  result = result.replace(/\*([^*\n]+)\*/g, '<em>$1</em>');
  // 裸链接
  result = result.replace(
    /(^|[\s(])(https?:\/\/[^\s<)]+)/g,
    (_match, prefix: string, url: string) =>
      `${prefix}${stash(
        `<a href="${url}" target="_blank" rel="noopener noreferrer">${url}</a>`
      )}`
  );

  return result.replace(
    new RegExp(`${tokenStart}(\\d+)${tokenEnd}`, 'g'),
    (_match, index: string) => tokens[Number(index)]
  );
}

/**
 * 渲染一个段落：段内的单换行保留成换行，与聊天场景的阅读习惯保持一致
 * @param lines 段落内的行
 */
function renderParagraph(lines: string[]): string {
  return `<p>${lines.map(renderInline).join('<br />')}</p>`;
}

/** 一个列表项 */
interface ListItem {
  /** 嵌套层级，2 个空格算一级 */
  depth: number;
  /** 是否有序列表 */
  ordered: boolean;
  /** 列表项文本 */
  text: string;
}

/**
 * 渲染列表项为嵌套的 ul / ol
 * @param items 列表项
 */
function renderList(items: ListItem[]): string {
  /** 已打开的列表层级 */
  const stack: Array<{ depth: number; ordered: boolean }> = [];
  const html: string[] = [];

  /** 打开一层列表 */
  const open = (item: ListItem) => {
    html.push(item.ordered ? '<ol>' : '<ul>');
    stack.push({ depth: item.depth, ordered: item.ordered });
  };

  /** 关闭一层列表 */
  const close = () => {
    const level = stack.pop();
    html.push(level?.ordered ? '</li></ol>' : '</li></ul>');
  };

  for (const item of items) {
    while (stack.length && item.depth < stack[stack.length - 1].depth) {
      close();
    }
    const top = stack[stack.length - 1];
    if (!top || item.depth > top.depth) {
      open(item);
    } else if (top.ordered !== item.ordered) {
      close();
      open(item);
    } else {
      html.push('</li>');
    }
    html.push(`<li>${renderInline(item.text)}`);
  }
  while (stack.length) {
    close();
  }
  return html.join('');
}

/**
 * 拆分表格行
 * @param line 行内容
 */
function splitTableCell(line: string): string[] {
  return line
    .trim()
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map((cell) => cell.trim());
}

/**
 * 渲染表格
 * @param lines 全部行
 * @param start 表头行号
 */
function collectTable(
  lines: string[],
  start: number
): { html: string; next: number } {
  const header = splitTableCell(lines[start]);
  const rows: string[][] = [];
  let index = start + 2;
  while (
    index < lines.length &&
    lines[index].includes('|') &&
    lines[index].trim()
  ) {
    rows.push(splitTableCell(lines[index]));
    index += 1;
  }
  const head = header.map((cell) => `<th>${renderInline(cell)}</th>`).join('');
  const body = rows
    .map(
      (row) =>
        `<tr>${row.map((cell) => `<td>${renderInline(cell)}</td>`).join('')}</tr>`
    )
    .join('');
  return {
    html: `<table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`,
    next: index
  };
}

/**
 * 收集从 start 开始的列表项
 * @param lines 全部行
 * @param start 起始行号
 */
function collectList(
  lines: string[],
  start: number
): { items: ListItem[]; next: number } {
  const items: ListItem[] = [];
  let index = start;
  while (index < lines.length) {
    const line = lines[index];
    const match = listItemPattern.exec(line);
    if (match) {
      items.push({
        depth: Math.floor(match[1].length / 2),
        ordered: /\d/.test(match[2]),
        text: match[3].trim()
      });
      index += 1;
      continue;
    }
    // 列表项的续行：并到上一项里
    if (line.trim() && !isBlockStart(line) && items.length) {
      items[items.length - 1].text += `\n${line.trim()}`;
      index += 1;
      continue;
    }
    break;
  }
  return { items, next: index };
}

/**
 * 把 Markdown 文本渲染成 HTML
 * @param text 消息内容
 */
export function renderMarkdown(text: string): string {
  const lines = (text ?? '').replace(/\r\n?/g, '\n').split('\n');
  const html: string[] = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];
    if (!line.trim()) {
      index += 1;
      continue;
    }
    // 代码块，内容不参与行内渲染
    const fence = fencePattern.exec(line);
    if (fence) {
      const body: string[] = [];
      index += 1;
      while (index < lines.length && !fenceEndPattern.test(lines[index])) {
        body.push(lines[index]);
        index += 1;
      }
      index += 1;
      const lang = fence[1];
      html.push(
        `<pre><code${
          lang ? ` class="language-${lang}"` : ''
        }>${escapeHtml(body.join('\n'))}</code></pre>`
      );
      continue;
    }
    // 标题
    const heading = headingPattern.exec(line);
    if (heading) {
      const level = heading[1].length;
      html.push(`<h${level}>${renderInline(heading[2].trim())}</h${level}>`);
      index += 1;
      continue;
    }
    // 分隔线
    if (dividerPattern.test(line)) {
      html.push('<hr />');
      index += 1;
      continue;
    }
    // 引用
    if (quotePattern.test(line)) {
      const body: string[] = [];
      while (index < lines.length && quotePattern.test(lines[index])) {
        body.push(lines[index].replace(quotePattern, ''));
        index += 1;
      }
      html.push(`<blockquote>${renderParagraph(body)}</blockquote>`);
      continue;
    }
    // 表格：表头行 + 分隔行
    if (
      line.includes('|') &&
      index + 1 < lines.length &&
      tableDividerPattern.test(lines[index + 1]) &&
      lines[index + 1].includes('-')
    ) {
      const table = collectTable(lines, index);
      html.push(table.html);
      index = table.next;
      continue;
    }
    // 列表
    if (listItemPattern.test(line)) {
      const list = collectList(lines, index);
      html.push(renderList(list.items));
      index = list.next;
      continue;
    }
    // 段落
    const body: string[] = [];
    while (
      index < lines.length &&
      lines[index].trim() &&
      (!body.length || !isBlockStart(lines[index]))
    ) {
      body.push(lines[index]);
      index += 1;
    }
    html.push(renderParagraph(body));
  }

  return html.join('');
}
