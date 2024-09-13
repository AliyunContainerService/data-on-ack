<template>
  <div class="dashboard-container">
    <el-tabs v-model="activeName" type="border-card" @tab-click="handleClick">
      <el-tab-pane label="集群" name="cluster" style="height:1000px; margin:-15px">
        <iframe :src="clusterUrl" frameBorder="0" scrolling="auto" style="height:100%; width:100%; margin-left:-60px;" />
      </el-tab-pane>
      <el-tab-pane label="节点" name="node" style="height:1000px; margin:-15px">
        <iframe :src="nodeUrl" frameBorder="0" scrolling="auto" style="height:100%; width:100%; margin-left:-60px;" />
      </el-tab-pane>
      <el-tab-pane label="任务" name="job" style="height:1000px; margin:-15px">
        <iframe :src="jobUrl" frameBorder="0" scrolling="auto" style="height:100%; width:100%; margin-left:-60px;" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script>
import { mapGetters } from 'vuex'
import { fetchDashboardUrl } from '@/api/dashboard'

export default {
  name: 'Dashboard',
  data() {
    return {
      activeName: 'cluster',
      clusterUrl: '',
      nodeUrl: '',
      jobUrl: ''
    }
  },
  computed: {
    ...mapGetters([
      'name',
      'roles'
    ])
  },
  created() {
    this.getDashboardUrl()
  },
  methods: {
    getDashboardUrl() {
      fetchDashboardUrl().then(response => {
        const baseUrl = response.data
        this.clusterUrl = baseUrl + '/d/kube-ai-cluster-details/kube-ai-cluster-details?orgId=1&refresh=10s'
        this.nodeUrl = baseUrl + '/d/kube-ai-node-details/kube-ai-node-details?orgId=1&refresh=10s'
        this.jobUrl = baseUrl + '/d/kube-ai-training-job-details/kube-ai-training-job-details?orgId=1&refresh=10s'
      }).catch(function(e) {
        // console.log(e)
      })
    },
    handleClick(tab, event) {
      // console.log(tab, event)
    }
  }

}
</script>

<style lang="scss" scoped>
.dashboard {
  &-container {
    margin: 0px
  }
  &-text {
    font-size: 30px;
    line-height: 46px;
  }
}
</style>
