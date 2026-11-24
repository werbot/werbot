<script lang="ts">
	import StepUpModal from '#lib/StepUpModal.svelte';
	import { stepUp } from '#lib/stepup.svelte.js';
</script>

<!-- the modal mounts per step-up, so each one asks the server for its factors afresh -->
{#if stepUp.open}<StepUpModal />{/if}
