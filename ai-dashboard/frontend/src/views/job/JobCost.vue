<template>
  <div class="app-container">
    <el-row :gutter="40" class="panel-group">
      <el-col :xs="12" :sm="12" :lg="6" class="card-panel-col">
        <div class="card-panel">
          <div class="card-panel-icon-wrapper icon-duration">
            <svg-icon icon-class="dashboard" class-name="card-panel-icon" />
          </div>
          <div class="card-panel-description">
            <div class="card-panel-text">
              {{ $t('job.duration') }}
            </div>
            <span class="card-panel-num">{{ jobCost.formatTime }}</span>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :lg="6" class="card-panel-col">
        <div class="card-panel">
          <div class="card-panel-icon-wrapper icon-realcost">
            <svg-icon icon-class="shopping" class-name="card-panel-icon" />
          </div>
          <div class="card-panel-description">
            <div class="card-panel-text">
              {{ $t('job.tradeCost') }}
            </div>
            <span class="card-panel-num">{{ jobCost.tradeCost }}</span>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :lg="6" class="card-panel-col">
        <div class="card-panel">
          <div class="card-panel-icon-wrapper icon-ondemandcost">
            <svg-icon icon-class="chart" class-name="card-panel-icon" />
          </div>
          <div class="card-panel-description">
            <div class="card-panel-text">
              {{ $t('job.onDemandCost') }}
            </div>
            <span class="card-panel-num">{{ jobCost.onDemandCost }}</span>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :lg="6" class="card-panel-col">
        <div class="card-panel">
          <div class="card-panel-icon-wrapper icon-savedmoney">
            <svg-icon icon-class="money" class-name="card-panel-icon" />
          </div>
          <div class="card-panel-description">
            <div class="card-panel-text">
              {{ $t('job.savedCost') }}
            </div>
            <span class="card-panel-num">{{ jobCost.savedCost*100 }}%</span>
          </div>
        </div>
      </el-col>
    </el-row>

    <el-table
      v-loading="listLoading"
      :data="jobCost.instances"
      element-loading-text="Loading"
      border
      fit
      highlight-current-row
    >
      <el-table-column align="center" label="ID">
        <template slot-scope="scope">
          {{ scope.$index+1 }}
        </template>
      </el-table-column>
      <el-table-column label="Pod名称" align="center">
        <template slot-scope="scope">
          <span>{{ scope.row.name }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.namespace')">
        <template slot-scope="scope">
          {{ scope.row.namespace }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.status')">
        <template slot-scope="scope">
          {{ scope.row.status }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.duration')" align="center">
        <template slot-scope="scope">
          {{ scope.row.formatTime }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.cpu')" align="center">
        <template slot-scope="scope">
          {{ scope.row.cpuCore }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.gpu')" align="center">
        <template slot-scope="scope">
          {{ scope.row.gpu }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.resourceType')" align="center">
        <template slot-scope="scope">
          {{ scope.row.resourceType }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.instanceType')" align="center">
        <template slot-scope="scope">
          {{ scope.row.instanceType }}
        </template>
      </el-table-column>
      <!--
      <el-table-column label="是否Spot" align="center">
        <template slot-scope="scope">
          {{ scope.row.spot }}
        </template>
      </el-table-column>
      -->
      <el-table-column :label="$t('job.tradePrice')" align="center">
        <template slot-scope="scope">
          {{ scope.row.tradePrice }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.onDemandPrice')" align="center">
        <template slot-scope="scope">
          {{ scope.row.onDemandPrice }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.tradeCost')" align="center">
        <template slot-scope="scope">
          {{ scope.row.tradeCost }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.onDemandCost')" align="center">
        <template slot-scope="scope">
          {{ scope.row.onDemandCost }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('job.savedCost')" align="center">
        <template slot-scope="scope">
          {{ scope.row.savedCost*100 }}%
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
import { fetchJobCost } from '@/api/job'

export default {
  data() {
    return {
      query: '',
      listLoading: false,
      jobCost: {},
      list: [],
      timer: null
    }
  },
  mounted() {
    this.query = this.$route.query
    this.getCost(this.query)
    this.timer = setInterval(() => {
      setTimeout(this.getCost(this.query), 0)
    }, 60000)
  },
  beforeDestroy() {
    clearInterval(this.timer)
    this.timer = null
  },
  methods: {
    getCost(query) {
      this.listLoading = true
      fetchJobCost(this.query).then(response => {
        this.listLoading = false
        this.jobCost = response.data
      })
    }
  }
}
</script>

<style lang="scss" scoped>
html, body {
  height: 100%;
}

.app-container {
  padding: 20px;
  background-color: rgb(240, 242, 245);
  position: relative;
}

.panel-group {
  margin-top: 18px;

  .card-panel-col {
    margin-bottom: 32px;
  }

  .card-panel {
    height: 108px;
    cursor: pointer;
    font-size: 12px;
    position: relative;
    overflow: hidden;
    color: #666;
    background: #fff;
    box-shadow: 4px 4px 40px rgba(0, 0, 0, .05);
    border-color: rgba(0, 0, 0, .05);

    &:hover {
      .card-panel-icon-wrapper {
        color: #fff;
      }

      .icon-duration {
        background: #40c9c6;
      }

      .icon-realcost {
        background: #36a3f7;
      }

      .icon-ondemandcost {
        background: #f4516c;
      }

      .icon-savedmoney {
        background: #34bfa3
      }
    }

    .icon-duration {
      color: #40c9c6;
    }

    .icon-realcost {
      color: #36a3f7;
    }

    .icon-ondemandcost {
      color: #f4516c;
    }

    .icon-savedmoney {
      color: #34bfa3
    }

    .card-panel-icon-wrapper {
      float: left;
      margin: 14px 0 0 14px;
      padding: 16px;
      transition: all 0.38s ease-out;
      border-radius: 6px;
    }

    .card-panel-icon {
      float: left;
      font-size: 48px;
    }

    .card-panel-description {
      float: right;
      font-weight: bold;
      margin: 26px;
      margin-left: 0px;

      .card-panel-text {
        line-height: 18px;
        color: rgba(0, 0, 0, 0.45);
        font-size: 16px;
        margin-bottom: 12px;
      }

      .card-panel-num {
        font-size: 20px;
      }
    }
  }
}

@media (max-width:550px) {
  .card-panel-description {
    display: none;
  }

  .card-panel-icon-wrapper {
    float: none !important;
    width: 100%;
    height: 100%;
    margin: 0 !important;

    .svg-icon {
      display: block;
      margin: 14px auto !important;
      float: none !important;
    }
  }
}
</style>
