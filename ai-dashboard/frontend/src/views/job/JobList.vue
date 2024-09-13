<template>
  <div class="app-container">
    <el-tabs v-model="activeName" type="border-card" @tab-click="handleClick">
      <el-tab-pane :label="$t('job.training')" name="training">
        <training-job-list />
      </el-tab-pane>
      <el-tab-pane :label="$t('job.inference')" name="serving">
        <serving-job-list />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
<script>
import TrainingJobList from './components/TrainingJobList'
import ServingJobList from './components/ServingJobList'

export default {
  name: 'JobList',
  components: {
    TrainingJobList,
    ServingJobList
  },
  data() {
    return {
      activeName: 'training'
    }
  },
  methods: {
    handleClick(tab, event) {
    }
  }
}
</script>

<style scoped>
  .app-container {
    margin: 5px;
  }
</style>
