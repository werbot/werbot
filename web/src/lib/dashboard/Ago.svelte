<script lang="ts">
	import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt';
	import { ago, fmt } from '#lib/format.js';

	let { ts }: { ts?: Timestamp } = $props();
</script>

<time datetime={ts && timestampDate(ts).toISOString()} title={fmt(ts)}>{ago(ts)}</time>
