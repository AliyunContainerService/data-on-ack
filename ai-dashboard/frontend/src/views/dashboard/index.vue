<template>
  <div class="dashboard-container">
    <div>
      <iframe :src="grafanaUrl" frameBorder="0" scrolling="auto" style="overflow: hidden; height: 100%; width: 100%; position: absolute; margin-left:-60px;" />
    </div>
  </div>
</template>

<script>
import { mapGetters } from 'vuex'

export default {
  name: 'Dashboard',
  data() {
    return {
      grafanaUrl: '/grafana/d/kube-ai-cluster-details/kube-ai-cluster-details?orgId=1&refresh=10s'
    }
  },
  computed: {
    ...mapGetters([
      'name',
      'roles'
    ])
  },
  created() {
  },
  methods: {
  }

}
</script>

<style lang="scss" scoped>
.dashboard {
  &-container {
    margin: 0px;
  }
  &-text {
    font-size: 30px;
    line-height: 46px;
  }
}
</style>
