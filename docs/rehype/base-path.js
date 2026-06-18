import { visit } from 'unist-util-visit';

/** Prepend Astro base path to root-relative links in authored markdown. */
export function rehypeBasePath(base = '/') {
	const prefix = base.replace(/\/$/, '');

	return (tree) => {
		visit(tree, 'element', (node) => {
			if (node.tagName !== 'a' || typeof node.properties?.href !== 'string') {
				return;
			}
			const href = node.properties.href;
			if (!href.startsWith('/') || href.startsWith('//')) {
				return;
			}
			if (href === prefix || href.startsWith(`${prefix}/`)) {
				return;
			}
			node.properties.href = `${prefix}${href}`;
		});
	};
}
