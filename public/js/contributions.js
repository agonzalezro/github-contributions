angular.module('GithubContributions', [])
  .controller('ContributionsController', ['$scope', '$http', function($scope, $http){ 
    $scope.get = function () {
      $http.get("/" + this.username).success(function(data){
        $scope.contributions = data;
        console.log(data);
      });
    };

    $scope.contributions = [];
  }]);
