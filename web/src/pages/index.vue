<template>
  <div class="artboard">
    <header>
      <h1>Shared servers</h1>
    </header>

    <table>
      <thead>
        <tr>
          <th class="w-20">#</th>
          <th>First</th>
          <th>Last</th>
          <th>Handle</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>1</td>
          <td>Mark</td>
          <td>Otto</td>
          <td>@mdo</td>
        </tr>
        <tr>
          <td>1</td>
          <td>Mark</td>
          <td>Otto</td>
          <td>@mdo</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
document.title = "servers shared with me";
</script>

<route lang="yaml">
meta:
  layout: private_blank
  requiresAuth: true
</route>
  